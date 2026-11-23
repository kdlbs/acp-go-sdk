package v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
)

type RequestPermissionRequest struct {
	SessionID   SessionID          `json:"sessionId"`
	Title       string             `json:"title"`
	Description *string            `json:"description,omitempty"`
	Subject     *PermissionSubject `json:"subject,omitempty"`
	Options     []PermissionOption `json:"options"`
	Meta        json.RawMessage    `json:"_meta,omitempty"`
}

func (r *RequestPermissionRequest) UnmarshalJSON(data []byte) error {
	type fields RequestPermissionRequest
	value, err := decodeObject[fields](data, "sessionId", "title", "options")
	if err != nil {
		return err
	}
	if len(value.Options) == 0 {
		return errors.New("expected at least one permission option")
	}
	*r = RequestPermissionRequest(value)
	return nil
}

type PermissionSubject struct {
	Type       string          `json:"type"`
	ToolCall   *ToolCallUpdate `json:"toolCall,omitempty"`
	Command    *string         `json:"command,omitempty"`
	Cwd        *string         `json:"cwd,omitempty"`
	ToolCallID *ToolCallID     `json:"toolCallId,omitempty"`
	TerminalID *TerminalID     `json:"terminalId,omitempty"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

func (s *PermissionSubject) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	var required []string
	switch kind {
	case "tool_call":
		required = []string{"toolCall"}
	case "command":
		required = []string{"command", "cwd"}
	default:
		*s = PermissionSubject{Type: kind, Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	if err := obj.require(required...); err != nil {
		return err
	}
	type fields PermissionSubject
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s = PermissionSubject(value)
	return nil
}

func (s PermissionSubject) MarshalJSON() ([]byte, error) {
	type fields PermissionSubject
	return marshalRaw(s.Raw, fields(s))
}

type PermissionOptionKind string

const (
	PermissionAllowOnce    PermissionOptionKind = "allow_once"
	PermissionAllowAlways  PermissionOptionKind = "allow_always"
	PermissionRejectOnce   PermissionOptionKind = "reject_once"
	PermissionRejectAlways PermissionOptionKind = "reject_always"
)

type PermissionOption struct {
	OptionID PermissionOptionID   `json:"optionId"`
	Name     string               `json:"name"`
	Kind     PermissionOptionKind `json:"kind"`
	Meta     json.RawMessage      `json:"_meta,omitempty"`
}

func (o *PermissionOption) UnmarshalJSON(data []byte) error {
	type fields PermissionOption
	value, err := decodeObject[fields](data, "optionId", "name", "kind")
	if err == nil {
		*o = PermissionOption(value)
	}
	return err
}

type RequestPermissionResponse struct {
	Outcome PermissionOutcome `json:"outcome"`
	Meta    json.RawMessage   `json:"_meta,omitempty"`
}

type PermissionOutcome struct {
	Outcome  string              `json:"outcome"`
	OptionID *PermissionOptionID `json:"optionId,omitempty"`
	Meta     json.RawMessage     `json:"_meta,omitempty"`
	Raw      json.RawMessage     `json:"-"`
}

func SelectedPermission(optionID PermissionOptionID) PermissionOutcome {
	return PermissionOutcome{Outcome: "selected", OptionID: &optionID}
}

func CancelledPermission() PermissionOutcome { return PermissionOutcome{Outcome: "cancelled"} }

func (o *PermissionOutcome) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("outcome")
	if err != nil {
		return err
	}
	if kind == "selected" {
		if err := obj.require("optionId"); err != nil {
			return err
		}
	}
	if kind != "selected" && kind != "cancelled" {
		*o = PermissionOutcome{Outcome: kind, Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	type fields PermissionOutcome
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*o = PermissionOutcome(value)
	return nil
}

func (o PermissionOutcome) MarshalJSON() ([]byte, error) {
	type fields PermissionOutcome
	return marshalRaw(o.Raw, fields(o))
}

type ElicitationMode string

const (
	ElicitationModeForm ElicitationMode = "form"
	ElicitationModeURL  ElicitationMode = "url"
)

type CreateElicitationRequest struct {
	Mode            ElicitationMode    `json:"mode"`
	Message         string             `json:"message"`
	SessionID       *SessionID         `json:"sessionId,omitempty"`
	ToolCallID      *ToolCallID        `json:"toolCallId,omitempty"`
	RequestID       json.RawMessage    `json:"requestId,omitempty"` // An integer or a string.
	RequestedSchema *ElicitationSchema `json:"requestedSchema,omitempty"`
	ElicitationID   *ElicitationID     `json:"elicitationId,omitempty"`
	URL             *string            `json:"url,omitempty"`
	Meta            json.RawMessage    `json:"_meta,omitempty"`
	Raw             json.RawMessage    `json:"-"`
}

func (r *CreateElicitationRequest) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	mode, err := obj.requiredString("mode")
	if err != nil {
		return err
	}
	message, err := obj.requiredString("message")
	if err != nil {
		return err
	}
	_, session := obj["sessionId"]
	_, request := obj["requestId"]
	_, toolCall := obj["toolCallId"]
	if session == request || (request && toolCall) {
		return errors.New("expected either session scope or request scope for elicitation")
	}
	value := CreateElicitationRequest{Mode: ElicitationMode(mode), Message: message, Meta: obj["_meta"]}
	if session {
		id, err := obj.requiredString("sessionId")
		if err != nil {
			return err
		}
		sessionID := SessionID(id)
		value.SessionID = &sessionID
		if toolCall {
			if err := json.Unmarshal(obj["toolCallId"], &value.ToolCallID); err != nil {
				return err
			}
		}
	} else {
		value.RequestID = obj["requestId"]
		if err := validateElicitationRequestID(value.RequestID); err != nil {
			return err
		}
	}
	switch value.Mode {
	case ElicitationModeForm:
		if err := obj.require("requestedSchema"); err != nil {
			return err
		}
		if err := json.Unmarshal(obj["requestedSchema"], &value.RequestedSchema); err != nil {
			return err
		}
	case ElicitationModeURL:
		id, err := obj.requiredString("elicitationId")
		if err != nil {
			return err
		}
		address, err := obj.requiredString("url")
		if err != nil {
			return err
		}
		uri, err := url.Parse(address)
		if err != nil || !uri.IsAbs() {
			return errors.New("expected an absolute elicitation URL")
		}
		elicitationID := ElicitationID(id)
		value.ElicitationID = &elicitationID
		value.URL = &address
	default:
		value.Raw = bytes.Clone(data)
	}
	*r = value
	return nil
}

func validateElicitationRequestID(raw json.RawMessage) error {
	if raw[0] == '"' {
		return nil
	}
	var number *int64
	if err := json.Unmarshal(raw, &number); err != nil || number == nil {
		return errors.New("expected an integer or string requestId")
	}
	return nil
}

func (r CreateElicitationRequest) MarshalJSON() ([]byte, error) {
	type fields CreateElicitationRequest
	return marshalRaw(r.Raw, fields(r))
}

type ElicitationSchema struct {
	Type        string                 `json:"type"`
	Properties  map[string]TaggedValue `json:"properties"`
	Required    []string               `json:"required,omitempty"`
	Title       *string                `json:"title,omitempty"`
	Description *string                `json:"description,omitempty"`
	Meta        json.RawMessage        `json:"_meta,omitempty"`
}

func (s *ElicitationSchema) UnmarshalJSON(data []byte) error {
	type fields ElicitationSchema
	value, err := decodeObject[fields](data, "type", "properties")
	if err != nil {
		return err
	}
	if value.Type != "object" {
		return errors.New("expected elicitation schema type object")
	}
	*s = ElicitationSchema(value)
	return nil
}

type ElicitationAction string

const (
	ElicitationAccept  ElicitationAction = "accept"
	ElicitationDecline ElicitationAction = "decline"
	ElicitationCancel  ElicitationAction = "cancel"
)

type CreateElicitationResponse struct {
	Action  ElicitationAction          `json:"action"`
	Content map[string]json.RawMessage `json:"content,omitempty"`
	Meta    json.RawMessage            `json:"_meta,omitempty"`
}

type CompleteElicitationNotification struct {
	ElicitationID ElicitationID   `json:"elicitationId"`
	Meta          json.RawMessage `json:"_meta,omitempty"`
}

func (n *CompleteElicitationNotification) UnmarshalJSON(data []byte) error {
	type fields CompleteElicitationNotification
	value, err := decodeObject[fields](data, "elicitationId")
	if err == nil {
		*n = CompleteElicitationNotification(value)
	}
	return err
}
