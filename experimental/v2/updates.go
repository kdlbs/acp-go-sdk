package v2

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/coder/acp-go-sdk"
)

type UpdateSessionNotification struct {
	SessionID SessionID       `json:"sessionId"`
	Update    SessionUpdate   `json:"update"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

func (n *UpdateSessionNotification) UnmarshalJSON(data []byte) error {
	type fields UpdateSessionNotification
	value, err := decodeObject[fields](data, "sessionId", "update")
	if err == nil {
		*n = UpdateSessionNotification(value)
	}
	return err
}

type SessionUpdateType string

const (
	UpdateUserMessage            SessionUpdateType = "user_message"
	UpdateUserMessageChunk       SessionUpdateType = "user_message_chunk"
	UpdateAgentMessage           SessionUpdateType = "agent_message"
	UpdateAgentMessageChunk      SessionUpdateType = "agent_message_chunk"
	UpdateAgentThought           SessionUpdateType = "agent_thought"
	UpdateAgentThoughtChunk      SessionUpdateType = "agent_thought_chunk"
	UpdateState                  SessionUpdateType = "state_update"
	UpdateReplayComplete         SessionUpdateType = "replay_complete"
	UpdateToolCall               SessionUpdateType = "tool_call_update"
	UpdateToolCallContentChunk   SessionUpdateType = "tool_call_content_chunk"
	UpdateTerminal               SessionUpdateType = "terminal_update"
	UpdateTerminalOutputChunk    SessionUpdateType = "terminal_output_chunk"
	UpdateSubagent               SessionUpdateType = "subagent_update"
	UpdateSessionMessage         SessionUpdateType = "session_message"
	UpdateSessionMessageChunk    SessionUpdateType = "session_message_chunk"
	UpdateConfigOption           SessionUpdateType = "config_option_update"
	UpdateAvailableCommands      SessionUpdateType = "available_commands_update"
	UpdateSessionInfo            SessionUpdateType = "session_info_update"
	UpdatePlan                   SessionUpdateType = "plan_update"
	UpdatePlanRemoved            SessionUpdateType = "plan_removed"
	UpdateUsage                  SessionUpdateType = "usage_update"
	UpdateNotice                 SessionUpdateType = "notice"
	UpdateCompaction             SessionUpdateType = "compaction_update"
	UpdateCompactionSummaryChunk SessionUpdateType = "compaction_summary_chunk"
)

// SessionUpdate is a union of the typed update payloads in this package.
// Decode stores a pointer to the matching type in Value.
type SessionUpdate struct {
	Value SessionUpdateValue
}

type SessionUpdateValue interface {
	updateType() SessionUpdateType
}

// Type returns the value of the wire discriminator, sessionUpdate.
func (u SessionUpdate) Type() SessionUpdateType {
	if u.Value == nil {
		return ""
	}
	return u.Value.updateType()
}

// UnknownSessionUpdate preserves a future variant as a complete JSON object.
type UnknownSessionUpdate struct {
	Kind SessionUpdateType
	Raw  json.RawMessage
}

func (u UnknownSessionUpdate) updateType() SessionUpdateType { return u.Kind }

func (u UnknownSessionUpdate) MarshalJSON() ([]byte, error) { return u.Raw, nil }

func (u *SessionUpdate) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	tag, err := obj.requiredString("sessionUpdate")
	if err != nil {
		return err
	}
	kind := SessionUpdateType(tag)
	var value SessionUpdateValue
	var required []string
	switch kind {
	case UpdateUserMessage, UpdateAgentMessage, UpdateAgentThought:
		value, required = &MessageUpdate{Kind: kind}, []string{"messageId"}
	case UpdateUserMessageChunk, UpdateAgentMessageChunk, UpdateAgentThoughtChunk, UpdateSessionMessageChunk:
		value, required = &MessageChunk{Kind: kind}, []string{"messageId", "content"}
	case UpdateState:
		value = &StateUpdate{}
	case UpdateReplayComplete:
		value = &ReplayComplete{}
	case UpdateToolCall:
		value = &ToolCallUpdate{}
	case UpdateToolCallContentChunk:
		value, required = &ToolCallContentChunk{}, []string{"toolCallId", "content"}
	case UpdateTerminal:
		value, required = &TerminalUpdate{}, []string{"terminalId"}
	case UpdateTerminalOutputChunk:
		value, required = &TerminalOutputChunk{}, []string{"terminalId", "data"}
	case UpdateSubagent:
		value, required = &SubagentUpdate{}, []string{"sessionId"}
	case UpdateSessionMessage:
		value, required = &SessionMessage{}, []string{"messageId"}
	case UpdateConfigOption:
		value, required = &ConfigOptionUpdate{}, []string{"configOptions"}
	case UpdateAvailableCommands:
		value, required = &AvailableCommandsUpdate{}, []string{"availableCommands"}
	case UpdateSessionInfo:
		value = &SessionInfoUpdate{}
	case UpdatePlan:
		value, required = &PlanUpdate{}, []string{"plan"}
	case UpdatePlanRemoved:
		value, required = &PlanRemoved{}, []string{"planId"}
	case UpdateUsage:
		value, required = &UsageUpdate{}, []string{"used", "size"}
	case UpdateNotice:
		value, required = &Notice{}, []string{"severity", "title"}
	case UpdateCompaction:
		value, required = &CompactionUpdate{}, []string{"compactionId"}
	case UpdateCompactionSummaryChunk:
		value, required = &CompactionSummaryChunk{}, []string{"compactionId", "content"}
	default:
		u.Value = &UnknownSessionUpdate{Kind: kind, Raw: bytes.Clone(data)}
		return nil
	}
	if err := obj.require(required...); err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}
	u.Value = value
	return nil
}

func (u SessionUpdate) MarshalJSON() ([]byte, error) {
	if u.Value == nil {
		return nil, errors.New("missing session update")
	}
	data, err := json.Marshal(u.Value)
	if err != nil {
		return nil, err
	}
	obj, err := readObject(data)
	if err != nil {
		return nil, err
	}
	obj["sessionUpdate"], _ = json.Marshal(u.Type())
	return json.Marshal(obj)
}

// MessageUpdate replaces content for a user message, agent message, or thought.
type MessageUpdate struct {
	Kind      SessionUpdateType             `json:"-"`
	MessageID MessageID                     `json:"messageId"`
	Content   acp.Optional[[]ContentBlock]  `json:"content"`
	Meta      acp.Optional[json.RawMessage] `json:"_meta"`
}

func (u MessageUpdate) updateType() SessionUpdateType { return u.Kind }

func (u MessageUpdate) MarshalJSON() ([]byte, error) {
	type fields MessageUpdate
	return marshalPatch(fields(u), map[string]bool{
		"content": u.Content.IsSet(),
		"_meta":   u.Meta.IsSet(),
	})
}

// MessageChunk appends content to a message or thought.
type MessageChunk struct {
	Kind      SessionUpdateType `json:"-"`
	MessageID MessageID         `json:"messageId"`
	Content   ContentBlock      `json:"content"`
	Meta      json.RawMessage   `json:"_meta,omitempty"`
}

func (u MessageChunk) updateType() SessionUpdateType { return u.Kind }

type SessionMessage struct {
	MessageID          MessageID                     `json:"messageId"`
	SenderSessionID    acp.Optional[SessionID]       `json:"senderSessionId"`
	RecipientSessionID acp.Optional[SessionID]       `json:"recipientSessionId"`
	SenderMessageID    acp.Optional[MessageID]       `json:"senderMessageId"`
	RecipientMessageID acp.Optional[MessageID]       `json:"recipientMessageId"`
	Content            acp.Optional[[]ContentBlock]  `json:"content"`
	Meta               acp.Optional[json.RawMessage] `json:"_meta"`
}

func (SessionMessage) updateType() SessionUpdateType { return UpdateSessionMessage }

func (u SessionMessage) MarshalJSON() ([]byte, error) {
	type fields SessionMessage
	return marshalPatch(fields(u), map[string]bool{
		"senderSessionId":    u.SenderSessionID.IsSet(),
		"recipientSessionId": u.RecipientSessionID.IsSet(),
		"senderMessageId":    u.SenderMessageID.IsSet(),
		"recipientMessageId": u.RecipientMessageID.IsSet(),
		"content":            u.Content.IsSet(),
		"_meta":              u.Meta.IsSet(),
	})
}

type SessionState string

const (
	StateRunning        SessionState = "running"
	StateIdle           SessionState = "idle"
	StateRequiresAction SessionState = "requires_action"
	StateUnknown        SessionState = "unknown"
)

type StopReason string

const (
	StopReasonEndTurn         StopReason = "end_turn"
	StopReasonMaxTokens       StopReason = "max_tokens"
	StopReasonMaxTurnRequests StopReason = "max_turn_requests"
	StopReasonRefusal         StopReason = "refusal"
	StopReasonCancelled       StopReason = "cancelled"
	StopReasonError           StopReason = "error"
)

type StateUpdate struct {
	State      SessionState                   `json:"state"`
	Background acp.Optional[[]BackgroundWork] `json:"background"`
	StopReason acp.Optional[StopReason]       `json:"stopReason"`
	Error      *StateError                    `json:"error,omitempty"`
	Meta       json.RawMessage                `json:"_meta,omitempty"`
	Raw        json.RawMessage                `json:"-"`
}

type StateError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (StateUpdate) updateType() SessionUpdateType { return UpdateState }

func (u *StateUpdate) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	state, err := obj.requiredString("state")
	if err != nil {
		return err
	}
	switch SessionState(state) {
	case StateIdle, StateRunning, StateRequiresAction, StateUnknown:
	default:
		*u = StateUpdate{State: SessionState(state), Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	type fields StateUpdate
	var value fields
	wire := struct {
		*fields
		Error json.RawMessage `json:"error"`
	}{fields: &value}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if reason, ok := value.StopReason.Get(); ok {
		switch reason {
		case StopReasonError:
			if wire.Error != nil {
				if err := json.Unmarshal(wire.Error, &value.Error); err != nil {
					return err
				}
			}
		case StopReasonEndTurn, StopReasonMaxTokens, StopReasonMaxTurnRequests, StopReasonRefusal, StopReasonCancelled:
		default:
			value.Raw = bytes.Clone(data)
		}
	}
	*u = StateUpdate(value)
	return nil
}

func (u StateUpdate) MarshalJSON() ([]byte, error) {
	if u.Raw != nil {
		return u.Raw, nil
	}
	type fields StateUpdate
	return marshalPatch(fields(u), map[string]bool{
		"background": u.Background.IsSet(),
		"stopReason": u.StopReason.IsSet(),
	})
}

type BackgroundWork struct {
	Type       string          `json:"type"`
	TerminalID *TerminalID     `json:"terminalId,omitempty"`
	ToolCallID *ToolCallID     `json:"toolCallId,omitempty"`
	SessionID  *SessionID      `json:"sessionId,omitempty"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

func (v *BackgroundWork) UnmarshalJSON(data []byte) error {
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
	case "terminal":
		required = "terminalId"
	case "tool_call":
		required = "toolCallId"
	case "subagent":
		required = "sessionId"
	default:
		*v = BackgroundWork{Type: kind, Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	if err := obj.require(required); err != nil {
		return err
	}
	type fields BackgroundWork
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = BackgroundWork(value)
	return nil
}

func (v BackgroundWork) MarshalJSON() ([]byte, error) {
	type fields BackgroundWork
	return marshalRaw(v.Raw, fields(v))
}

type ReplayComplete struct {
	Meta json.RawMessage `json:"_meta,omitempty"`
}

func (ReplayComplete) updateType() SessionUpdateType { return UpdateReplayComplete }

type SubagentUpdate struct {
	SessionID             SessionID                                 `json:"sessionId"`
	Title                 acp.Optional[string]                      `json:"title"`
	Description           acp.Optional[string]                      `json:"description"`
	Capabilities          acp.Optional[SubagentSessionCapabilities] `json:"capabilities"`
	State                 acp.Optional[StateUpdate]                 `json:"state"`
	Cwd                   acp.Optional[string]                      `json:"cwd"`
	AdditionalDirectories acp.Optional[[]string]                    `json:"additionalDirectories"`
	Meta                  acp.Optional[json.RawMessage]             `json:"_meta"`
}

type SubagentSessionCapabilities struct {
	Cancel *Capability     `json:"cancel,omitempty"`
	Meta   json.RawMessage `json:"_meta,omitempty"`
}

func (SubagentUpdate) updateType() SessionUpdateType { return UpdateSubagent }

func (u SubagentUpdate) MarshalJSON() ([]byte, error) {
	type fields SubagentUpdate
	return marshalPatch(fields(u), map[string]bool{
		"title":                 u.Title.IsSet(),
		"description":           u.Description.IsSet(),
		"capabilities":          u.Capabilities.IsSet(),
		"state":                 u.State.IsSet(),
		"cwd":                   u.Cwd.IsSet(),
		"additionalDirectories": u.AdditionalDirectories.IsSet(),
		"_meta":                 u.Meta.IsSet(),
	})
}
