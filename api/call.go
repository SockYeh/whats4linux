package api

import (
	"context"
	"errors"

	"github.com/lugvitc/whats4linux/internal/voip"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.mau.fi/whatsmeow/types"
)

var errVoIPNotReady = errors.New("voip is not ready")

type CallInfo struct {
	ID        string `json:"id"`
	Peer      string `json:"peer"`
	ChatJID   string `json:"chat_jid"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	State     string `json:"state"`
	InputID   string `json:"input_id"`
	OutputID  string `json:"output_id"`
}

func (a *Api) callInfoFrom(info voip.Info) CallInfo {
	name, chatJID := a.callPeerIdentity(info.Peer, info.ChatJID)
	if chatJID == "" {
		chatJID = info.ChatJID
	}
	return CallInfo{
		ID:        info.ID,
		Peer:      info.Peer,
		ChatJID:   chatJID,
		Name:      name,
		Direction: info.Direction,
		State:     info.State,
		InputID:   info.InputID,
		OutputID:  info.OutputID,
	}
}

func callContactHasName(info types.ContactInfo) bool {
	return info.FullName != "" || info.FirstName != "" || info.PushName != "" || info.BusinessName != ""
}

func callContactLabel(info types.ContactInfo) string {
	switch {
	case info.FullName != "":
		return info.FullName
	case info.FirstName != "":
		return info.FirstName
	case info.PushName != "":
		return info.PushName
	case info.BusinessName != "":
		return info.BusinessName
	default:
		return ""
	}
}

func (a *Api) callPeerIdentity(peer, chatJID string) (name, canonical string) {
	raw := chatJID
	if raw == "" {
		raw = peer
	}
	if raw == "" {
		return "", chatJID
	}
	jid, err := types.ParseJID(raw)
	if err != nil {
		return "", chatJID
	}
	if a == nil || a.waClient == nil || a.waClient.Store == nil {
		return "", jid.ToNonAD().String()
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	canonicalJID := jid.ToNonAD()
	if jid.Server == types.HiddenUserServer && a.waClient.Store.LIDs != nil {
		if pn, err := a.waClient.Store.LIDs.GetPNForLID(ctx, jid); err == nil && !pn.IsEmpty() {
			canonicalJID = pn.ToNonAD()
		}
	}
	canonical = canonicalJID.String()

	lookup := func(j types.JID) types.ContactInfo {
		if a.waClient.Store.Contacts == nil || j.IsEmpty() {
			return types.ContactInfo{}
		}
		info, err := a.waClient.Store.Contacts.GetContact(ctx, j.ToNonAD())
		if err != nil {
			return types.ContactInfo{}
		}
		return info
	}

	info := lookup(canonicalJID)
	if !callContactHasName(info) {
		info = lookup(jid)
	}
	return callContactLabel(info), canonical
}

func (a *Api) emitCall(info voip.Info) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "wa:call", a.callInfoFrom(info))
}

func (a *Api) PlaceCall(target string) error {
	if a.voip == nil {
		return errVoIPNotReady
	}
	return a.voip.Place(a.ctx, target)
}

func (a *Api) AnswerCall() error {
	if a.voip == nil {
		return errVoIPNotReady
	}
	return a.voip.Answer()
}

func (a *Api) RejectCall() error {
	if a.voip == nil {
		return errVoIPNotReady
	}
	return a.voip.Reject()
}

func (a *Api) HangUp() error {
	if a.voip == nil {
		return nil
	}
	return a.voip.Hangup()
}

func (a *Api) GetCallInfo() CallInfo {
	if a.voip == nil {
		return CallInfo{State: "idle"}
	}
	return a.callInfoFrom(a.voip.Info())
}

func (a *Api) ListAudioDevices() (voip.AudioDevices, error) {
	if a.voip == nil {
		return voip.ListAudioDevices()
	}
	return a.voip.Devices()
}

func (a *Api) SetAudioInput(id string) error {
	if a.voip == nil {
		return errVoIPNotReady
	}
	return a.voip.SetInput(id)
}

func (a *Api) SetAudioOutput(id string) error {
	if a.voip == nil {
		return errVoIPNotReady
	}
	return a.voip.SetOutput(id)
}
