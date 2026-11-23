// Package v2 implements the client side of the ACP v2 Preview protocol.
package v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const ProtocolVersionNumber = 2

// Agent method names.
const (
	AgentMethodInitialize             = "initialize"
	AgentMethodAuthLogin              = "auth/login"
	AgentMethodAuthLogout             = "auth/logout"
	AgentMethodSessionNew             = "session/new"
	AgentMethodSessionResume          = "session/resume"
	AgentMethodSessionList            = "session/list"
	AgentMethodSessionClose           = "session/close"
	AgentMethodSessionSetConfigOption = "session/set_config_option"
	AgentMethodSessionPrompt          = "session/prompt"
	AgentMethodSessionCancel          = "session/cancel"
)

// Client method names.
const (
	ClientMethodSessionUpdate            = "session/update"
	ClientMethodSessionRequestPermission = "session/request_permission"
	ClientMethodElicitationCreate        = "elicitation/create"
	ClientMethodElicitationComplete      = "elicitation/complete"
)

type wireObject map[string]json.RawMessage

func readObject(data []byte) (wireObject, error) {
	var obj wireObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("expected a JSON object")
	}
	return obj, nil
}

func (obj wireObject) require(names ...string) error {
	for _, name := range names {
		raw := obj[name]
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			return fmt.Errorf("missing or null %q", name)
		}
	}
	return nil
}

func (obj wireObject) requiredString(name string) (string, error) {
	if err := obj.require(name); err != nil {
		return "", err
	}
	var kind string
	if err := json.Unmarshal(obj[name], &kind); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return kind, nil
}

func decodeObject[T any](data []byte, required ...string) (T, error) {
	var value T
	obj, err := readObject(data)
	if err != nil {
		return value, err
	}
	if err := obj.require(required...); err != nil {
		return value, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}

func marshalRaw(raw json.RawMessage, value any) ([]byte, error) {
	if raw != nil {
		return raw, nil
	}
	return json.Marshal(value)
}

// Go 1.21 does not omit a zero-valued Optional struct with omitempty.
func marshalPatch(value any, present map[string]bool) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	obj, err := readObject(data)
	if err != nil {
		return nil, err
	}
	for name, set := range present {
		if !set {
			delete(obj, name)
		}
	}
	return json.Marshal(obj)
}

// TaggedValue preserves the discriminator and original object of a nested
// value whose payload the SDK leaves to the caller.
type TaggedValue struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (v *TaggedValue) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	*v = TaggedValue{Type: kind, Raw: bytes.Clone(data)}
	return nil
}

func (v TaggedValue) MarshalJSON() ([]byte, error) {
	type fields TaggedValue
	return marshalRaw(v.Raw, fields(v))
}
