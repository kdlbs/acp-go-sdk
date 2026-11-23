package v2

import (
	"bytes"
	"encoding/json"

	"github.com/coder/acp-go-sdk"
)

type ToolCallUpdate struct {
	ToolCallID   ToolCallID                         `json:"toolCallId"`
	Name         acp.Optional[string]               `json:"name"`
	Title        acp.Optional[string]               `json:"title"`
	Kind         acp.Optional[string]               `json:"kind"`
	Status       acp.Optional[string]               `json:"status"`
	Content      acp.Optional[[]ToolCallContent]    `json:"content"`
	Locations    acp.Optional[[]ToolCallLocation]   `json:"locations"`
	RawInput     acp.Optional[json.RawMessage]      `json:"rawInput"`
	RawOutput    acp.Optional[json.RawMessage]      `json:"rawOutput"`
	Capabilities acp.Optional[ToolCallCapabilities] `json:"capabilities"`
	Meta         acp.Optional[json.RawMessage]      `json:"_meta"`
}

func (ToolCallUpdate) updateType() SessionUpdateType { return UpdateToolCall }

func (u *ToolCallUpdate) UnmarshalJSON(data []byte) error {
	type fields ToolCallUpdate
	value, err := decodeObject[fields](data, "toolCallId")
	if err == nil {
		*u = ToolCallUpdate(value)
	}
	return err
}

func (u ToolCallUpdate) MarshalJSON() ([]byte, error) {
	type fields ToolCallUpdate
	return marshalPatch(fields(u), map[string]bool{
		"name":         u.Name.IsSet(),
		"title":        u.Title.IsSet(),
		"kind":         u.Kind.IsSet(),
		"status":       u.Status.IsSet(),
		"content":      u.Content.IsSet(),
		"locations":    u.Locations.IsSet(),
		"rawInput":     u.RawInput.IsSet(),
		"rawOutput":    u.RawOutput.IsSet(),
		"capabilities": u.Capabilities.IsSet(),
		"_meta":        u.Meta.IsSet(),
	})
}

type ToolCallCapabilities struct {
	Cancel *Capability     `json:"cancel,omitempty"`
	Meta   json.RawMessage `json:"_meta,omitempty"`
}

type ToolCallLocation struct {
	Path string          `json:"path"`
	Line *uint32         `json:"line,omitempty"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type ToolCallContentChunk struct {
	ToolCallID ToolCallID      `json:"toolCallId"`
	Content    ToolCallContent `json:"content"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
}

func (ToolCallContentChunk) updateType() SessionUpdateType { return UpdateToolCallContentChunk }

// ToolCallContent can reference a terminal or child session, or carry content.
// Raw holds diff payloads and variants the SDK leaves to the caller.
type ToolCallContent struct {
	Type       string          `json:"type"`
	Content    *ContentBlock   `json:"content,omitempty"`
	TerminalID *TerminalID     `json:"terminalId,omitempty"`
	SessionID  *SessionID      `json:"sessionId,omitempty"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

func (v *ToolCallContent) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	var required string
	switch kind {
	case "content":
		required = "content"
	case "terminal":
		required = "terminalId"
	case "session":
		required = "sessionId"
	case "diff":
		if err := obj.require("changes"); err != nil {
			return err
		}
		fallthrough
	default:
		*v = ToolCallContent{Type: kind, Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	if err := obj.require(required); err != nil {
		return err
	}
	type fields ToolCallContent
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = ToolCallContent(value)
	return nil
}

func (v ToolCallContent) MarshalJSON() ([]byte, error) {
	type fields ToolCallContent
	return marshalRaw(v.Raw, fields(v))
}

type TerminalUpdate struct {
	TerminalID   TerminalID                         `json:"terminalId"`
	Command      acp.Optional[string]               `json:"command"`
	Cwd          acp.Optional[string]               `json:"cwd"`
	Output       acp.Optional[TerminalOutput]       `json:"output"`
	ExitStatus   acp.Optional[TerminalExitStatus]   `json:"exitStatus"`
	Size         acp.Optional[TerminalSize]         `json:"size"`
	Capabilities acp.Optional[TerminalCapabilities] `json:"capabilities"`
	Meta         acp.Optional[json.RawMessage]      `json:"_meta"`
}

func (TerminalUpdate) updateType() SessionUpdateType { return UpdateTerminal }

func (u TerminalUpdate) MarshalJSON() ([]byte, error) {
	type fields TerminalUpdate
	return marshalPatch(fields(u), map[string]bool{
		"command":      u.Command.IsSet(),
		"cwd":          u.Cwd.IsSet(),
		"output":       u.Output.IsSet(),
		"exitStatus":   u.ExitStatus.IsSet(),
		"size":         u.Size.IsSet(),
		"capabilities": u.Capabilities.IsSet(),
		"_meta":        u.Meta.IsSet(),
	})
}

type TerminalOutput struct {
	Data string          `json:"data"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type TerminalExitStatus struct {
	ExitCode *int64          `json:"exitCode,omitempty"`
	Signal   *string         `json:"signal,omitempty"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

type TerminalSize struct {
	Cols uint16          `json:"cols"`
	Rows uint16          `json:"rows"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type TerminalCapabilities struct {
	Cancel *Capability     `json:"cancel,omitempty"`
	Meta   json.RawMessage `json:"_meta,omitempty"`
}

type TerminalOutputChunk struct {
	TerminalID TerminalID      `json:"terminalId"`
	Data       string          `json:"data"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
}

func (TerminalOutputChunk) updateType() SessionUpdateType { return UpdateTerminalOutputChunk }
