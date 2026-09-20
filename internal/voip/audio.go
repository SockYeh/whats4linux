package voip

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
	meowcaller "github.com/purpshell/meowcaller"
)

const numChannels = 1

type AudioDevice struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Default bool   `json:"default"`
}

type AudioDevices struct {
	Inputs   []AudioDevice `json:"inputs"`
	Outputs  []AudioDevice `json:"outputs"`
	InputID  string        `json:"input_id"`
	OutputID string        `json:"output_id"`
}

func ListAudioDevices() (AudioDevices, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return AudioDevices{}, err
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	inputs, err := collectDevices(ctx, malgo.Capture, "input")
	if err != nil {
		return AudioDevices{}, err
	}
	outputs, err := collectDevices(ctx, malgo.Playback, "output")
	if err != nil {
		return AudioDevices{}, err
	}
	return AudioDevices{Inputs: inputs, Outputs: outputs}, nil
}

func collectDevices(ctx *malgo.AllocatedContext, kind malgo.DeviceType, label string) ([]AudioDevice, error) {
	infos, err := ctx.Devices(kind)
	if err != nil {
		return nil, err
	}
	out := make([]AudioDevice, 0, len(infos)+1)
	out = append(out, AudioDevice{ID: "", Name: "Default", Kind: label, Default: true})
	for _, info := range infos {
		out = append(out, AudioDevice{
			ID:      info.ID.String(),
			Name:    info.Name(),
			Kind:    label,
			Default: info.IsDefault != 0,
		})
	}
	return out, nil
}

func lookupDeviceID(ctx *malgo.AllocatedContext, kind malgo.DeviceType, id string) (malgo.DeviceID, error) {
	if id == "" {
		return malgo.DeviceID{}, nil
	}
	infos, err := ctx.Devices(kind)
	if err != nil {
		return malgo.DeviceID{}, err
	}
	for _, info := range infos {
		if info.ID.String() == id {
			return info.ID, nil
		}
	}
	return malgo.DeviceID{}, fmt.Errorf("audio device %s not found", id)
}

func openMic(deviceID string) (meowcaller.AudioSource, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = numChannels
	cfg.SampleRate = meowcaller.SampleRate
	cfg.Alsa.NoMMap = 1
	var pinner runtime.Pinner
	id, err := lookupDeviceID(ctx, malgo.Capture, deviceID)
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	if deviceID != "" {
		pinner.Pin(&id)
		cfg.Capture.DeviceID = unsafe.Pointer(&id)
	}

	frames := make(chan []int16, 16)
	var acc []int16
	onData := func(_, in []byte, _ uint32) {
		for i := 0; i+1 < len(in); i += 2 {
			acc = append(acc, int16(binary.LittleEndian.Uint16(in[i:])))
		}
		for len(acc) >= meowcaller.FrameSamples {
			f := make([]int16, meowcaller.FrameSamples)
			copy(f, acc[:meowcaller.FrameSamples])
			acc = acc[meowcaller.FrameSamples:]
			select {
			case frames <- f:
			default:
			}
		}
	}

	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		pinner.Unpin()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	if err := dev.Start(); err != nil {
		pinner.Unpin()
		dev.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	return &micSource{ctx: ctx, dev: dev, frames: frames, pinner: &pinner}, nil
}

type micSource struct {
	ctx    *malgo.AllocatedContext
	dev    *malgo.Device
	frames <-chan []int16
	pinner *runtime.Pinner
	once   sync.Once
}

func (m *micSource) ReadFrame() ([]float32, error) {
	pcm, ok := <-m.frames
	if !ok {
		return nil, nil
	}
	return pcmToFloat(pcm), nil
}

func (m *micSource) Close() error {
	m.once.Do(func() {
		_ = m.dev.Stop()
		m.dev.Uninit()
		_ = m.ctx.Uninit()
		m.ctx.Free()
		if m.pinner != nil {
			m.pinner.Unpin()
		}
	})
	return nil
}

func openSpeaker(deviceID string) (meowcaller.AudioSink, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = malgo.FormatS16
	cfg.Playback.Channels = numChannels
	cfg.SampleRate = meowcaller.SampleRate
	var pinner runtime.Pinner
	id, err := lookupDeviceID(ctx, malgo.Playback, deviceID)
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	if deviceID != "" {
		pinner.Pin(&id)
		cfg.Playback.DeviceID = unsafe.Pointer(&id)
	}

	in := make(chan []int16, 64)
	var (
		mu  sync.Mutex
		buf []int16
	)
	done := make(chan struct{})
	go func() {
		for f := range in {
			mu.Lock()
			buf = append(buf, f...)
			mu.Unlock()
		}
		close(done)
	}()

	onData := func(out, _ []byte, count uint32) {
		need := int(count)
		mu.Lock()
		n := min(need, len(buf))
		for i := range n {
			binary.LittleEndian.PutUint16(out[i*2:], uint16(buf[i]))
		}
		buf = buf[n:]
		mu.Unlock()
		for i := n * 2; i < need*2; i++ {
			out[i] = 0
		}
	}

	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		pinner.Unpin()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	if err := dev.Start(); err != nil {
		pinner.Unpin()
		dev.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, err
	}
	return &speakerSink{ctx: ctx, dev: dev, in: in, done: done, pinner: &pinner}, nil
}

type speakerSink struct {
	ctx    *malgo.AllocatedContext
	dev    *malgo.Device
	in     chan []int16
	done   chan struct{}
	pinner *runtime.Pinner
	once   sync.Once
}

func (s *speakerSink) WriteFrame(frame []float32) error {
	s.in <- floatToPCM(frame)
	return nil
}

func (s *speakerSink) Close() error {
	s.once.Do(func() {
		close(s.in)
		<-s.done
		_ = s.dev.Stop()
		s.dev.Uninit()
		_ = s.ctx.Uninit()
		s.ctx.Free()
		if s.pinner != nil {
			s.pinner.Unpin()
		}
	})
	return nil
}

func pcmToFloat(pcm []int16) []float32 {
	out := make([]float32, len(pcm))
	for i, s := range pcm {
		out[i] = float32(s) / 32768
	}
	return out
}

func floatToPCM(f []float32) []int16 {
	out := make([]int16, len(f))
	for i, s := range f {
		v := s * 32768
		switch {
		case v > 32767:
			v = 32767
		case v < -32768:
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}
