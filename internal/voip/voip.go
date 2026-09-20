package voip

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	meowcaller "github.com/purpshell/meowcaller"
	"go.mau.fi/whatsmeow"
)

type Info struct {
	ID        string `json:"id"`
	Peer      string `json:"peer"`
	ChatJID   string `json:"chat_jid"`
	Direction string `json:"direction"`
	State     string `json:"state"`
	InputID   string `json:"input_id"`
	OutputID  string `json:"output_id"`
}

type History interface {
	CallStarted(id, peer, chatJID, direction, state string, startedAt int64)
	CallEnded(id, state, reason string, endedAt int64)
}

type Manager struct {
	client    *meowcaller.Client
	mu        sync.Mutex
	call      *meowcaller.Call
	player    *meowcaller.Player
	mic       meowcaller.AudioSource
	speaker   meowcaller.AudioSink
	direction string
	chatJID   string
	inputID   string
	outputID  string
	onChange  func(Info)
	history   History
}

func Attach(wa *whatsmeow.Client) *Manager {
	if wa == nil {
		return nil
	}
	m := &Manager{client: meowcaller.NewClient(wa)}
	m.client.OnIncomingCall(m.onIncoming)
	return m
}

func (m *Manager) SetOnChange(fn func(Info)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.onChange = fn
	m.mu.Unlock()
}

func (m *Manager) SetHistory(h History) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.history = h
	m.mu.Unlock()
}

func (m *Manager) Place(ctx context.Context, target string) error {
	if m == nil || m.client == nil {
		return errors.New("voip is not ready")
	}
	if err := m.ensureIdle(); err != nil {
		return err
	}
	call, err := m.client.Call(ctx, target)
	if err != nil {
		return err
	}
	if err := m.adopt(call, "outgoing", target); err != nil {
		_ = call.Hangup()
		return err
	}
	m.startAudio(call, true)
	log.Printf("voip: placed call %s to %s", call.ID(), call.Peer())
	return nil
}

func (m *Manager) Answer() error {
	call, err := m.current()
	if err != nil {
		return err
	}
	if err := call.Answer(); err != nil {
		return err
	}
	m.startAudio(call, false)
	log.Printf("voip: answered call %s", call.ID())
	m.notify()
	return nil
}

func (m *Manager) Reject() error {
	call, err := m.current()
	if err != nil {
		return err
	}
	if err := call.Reject(); err != nil {
		return err
	}
	log.Printf("voip: rejected call %s", call.ID())
	return nil
}

func (m *Manager) Hangup() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	call := m.call
	m.mu.Unlock()
	if call == nil {
		return nil
	}
	return call.Hangup()
}

func (m *Manager) Info() Info {
	if m == nil {
		return Info{State: "idle"}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.call == nil {
		return Info{State: "idle", InputID: m.inputID, OutputID: m.outputID}
	}
	return m.infoLocked(m.call)
}

func (m *Manager) Devices() (AudioDevices, error) {
	devs, err := ListAudioDevices()
	if err != nil {
		return AudioDevices{}, err
	}
	if m != nil {
		m.mu.Lock()
		devs.InputID = m.inputID
		devs.OutputID = m.outputID
		m.mu.Unlock()
	}
	return devs, nil
}

func (m *Manager) SetInput(id string) error {
	if m == nil {
		return errors.New("voip is not ready")
	}
	m.mu.Lock()
	m.inputID = id
	call := m.call
	m.mu.Unlock()
	if call == nil {
		m.notify()
		return nil
	}
	go m.rewireMicLogged(call)
	return nil
}

func (m *Manager) SetOutput(id string) error {
	if m == nil {
		return errors.New("voip is not ready")
	}
	m.mu.Lock()
	m.outputID = id
	call := m.call
	m.mu.Unlock()
	if call == nil {
		m.notify()
		return nil
	}
	go m.rewireSpeakerLogged(call)
	return nil
}

func (m *Manager) onIncoming(call *meowcaller.Call) {
	peer := call.Peer().String()
	if err := m.adopt(call, "incoming", peer); err != nil {
		log.Printf("voip: rejecting call %s from %s: %v", call.ID(), peer, err)
		_ = call.Reject()
		return
	}
	log.Printf("voip: incoming call %s from %s", call.ID(), peer)
}

func (m *Manager) ensureIdle() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.call != nil && m.call.State() != meowcaller.CallPhaseEnded {
		return errors.New("already in a call")
	}
	m.call = nil
	m.direction = ""
	m.chatJID = ""
	return nil
}

func (m *Manager) adopt(call *meowcaller.Call, direction, chatJID string) error {
	m.mu.Lock()
	if m.call != nil && m.call.State() != meowcaller.CallPhaseEnded {
		m.mu.Unlock()
		return errors.New("already in a call")
	}
	m.call = call
	m.direction = direction
	m.chatJID = chatJID
	hist := m.history
	m.mu.Unlock()
	m.watch(call)
	if hist != nil {
		hist.CallStarted(call.ID(), call.Peer().String(), chatJID, direction, phaseName(call.State()), time.Now().Unix())
	}
	m.notify()
	return nil
}

func (m *Manager) current() (*meowcaller.Call, error) {
	if m == nil {
		return nil, errors.New("voip is not ready")
	}
	m.mu.Lock()
	call := m.call
	m.mu.Unlock()
	if call == nil {
		return nil, errors.New("no active call")
	}
	return call, nil
}

func (m *Manager) watch(call *meowcaller.Call) {
	call.OnStateChange(func(phase meowcaller.CallPhase) {
		log.Printf("voip: call %s state %s", call.ID(), phaseName(phase))
		m.notify()
	})
	call.OnReady(func() {
		log.Printf("voip: call %s media flowing", call.ID())
		m.notify()
	})
	call.OnEnd(func(reason string) {
		log.Printf("voip: call %s ended: %s", call.ID(), reason)
		m.mu.Lock()
		hist := m.history
		if m.call == call {
			m.call = nil
			m.player = nil
			m.mic = nil
			m.speaker = nil
			m.direction = ""
			m.chatJID = ""
		}
		m.mu.Unlock()
		if hist != nil {
			hist.CallEnded(call.ID(), "ended", reason, time.Now().Unix())
		}
		m.notify()
	})
}

func (m *Manager) notify() {
	m.mu.Lock()
	fn := m.onChange
	call := m.call
	info := Info{State: "idle", InputID: m.inputID, OutputID: m.outputID}
	if call != nil {
		info = m.infoLocked(call)
	}
	m.mu.Unlock()
	if fn != nil {
		fn(info)
	}
}

func (m *Manager) infoLocked(call *meowcaller.Call) Info {
	return Info{
		ID:        call.ID(),
		Peer:      call.Peer().String(),
		ChatJID:   m.chatJID,
		Direction: m.direction,
		State:     phaseName(call.State()),
		InputID:   m.inputID,
		OutputID:  m.outputID,
	}
}

func phaseName(p meowcaller.CallPhase) string {
	switch p {
	case meowcaller.CallPhaseIdle:
		return "idle"
	case meowcaller.CallPhaseCalling:
		return "calling"
	case meowcaller.CallPhaseRinging:
		return "ringing"
	case meowcaller.CallPhaseConnecting:
		return "connecting"
	case meowcaller.CallPhaseActive:
		return "active"
	case meowcaller.CallPhaseEnded:
		return "ended"
	case meowcaller.CallPhaseWaitingRoom:
		return "waiting_room"
	default:
		return "unknown"
	}
}

func (m *Manager) startAudio(call *meowcaller.Call, hangupOnFail bool) {
	go func() {
		if err := m.rewireMic(call); err != nil {
			log.Printf("voip: microphone: %v", err)
			if hangupOnFail {
				_ = call.Hangup()
			}
			return
		}
		if err := m.rewireSpeaker(call); err != nil {
			log.Printf("voip: speaker: %v", err)
			if hangupOnFail {
				_ = call.Hangup()
			}
		}
	}()
}

func (m *Manager) rewireMicLogged(call *meowcaller.Call) {
	if err := m.rewireMic(call); err != nil {
		log.Printf("voip: microphone: %v", err)
	}
}

func (m *Manager) rewireSpeakerLogged(call *meowcaller.Call) {
	if err := m.rewireSpeaker(call); err != nil {
		log.Printf("voip: speaker: %v", err)
	}
}

func (m *Manager) stillCurrent(call *meowcaller.Call) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.call == call
}

func (m *Manager) rewireMic(call *meowcaller.Call) error {
	m.mu.Lock()
	id := m.inputID
	m.mu.Unlock()

	mic, err := openMic(id)
	if err != nil {
		return fmt.Errorf("open microphone: %w", err)
	}
	if !m.stillCurrent(call) {
		_ = mic.Close()
		return nil
	}

	m.mu.Lock()
	oldMic := m.mic
	oldPlayer := m.player
	m.mu.Unlock()
	if oldPlayer != nil {
		oldPlayer.Stop()
	}
	if oldMic != nil {
		_ = oldMic.Close()
	}
	player := call.Play(mic)
	m.mu.Lock()
	if m.call != call {
		m.mu.Unlock()
		player.Stop()
		_ = mic.Close()
		return nil
	}
	m.mic = mic
	m.player = player
	m.mu.Unlock()
	m.notify()
	return nil
}

func (m *Manager) rewireSpeaker(call *meowcaller.Call) error {
	m.mu.Lock()
	id := m.outputID
	m.mu.Unlock()

	speaker, err := openSpeaker(id)
	if err != nil {
		return fmt.Errorf("open speaker: %w", err)
	}
	if !m.stillCurrent(call) {
		_ = speaker.Close()
		return nil
	}

	call.Receive(speaker)
	m.mu.Lock()
	old := m.speaker
	if m.call != call {
		m.mu.Unlock()
		_ = speaker.Close()
		return nil
	}
	m.speaker = speaker
	m.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	m.notify()
	return nil
}
