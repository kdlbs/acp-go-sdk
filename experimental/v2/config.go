package v2

import (
	"bytes"
	"encoding/json"
	"errors"
)

// ConfigValue is the value to set on a session configuration option.
type ConfigValue interface {
	configValue() (string, any)
}

// SelectConfigValue identifies a choice for a select option. The request
// uses the wire discriminator "id" for this value.
type SelectConfigValue string

func (v SelectConfigValue) configValue() (string, any) { return "id", string(v) }

type BooleanConfigValue bool

func (v BooleanConfigValue) configValue() (string, any) { return "boolean", bool(v) }

type CustomConfigValue struct {
	Type  string
	Value json.RawMessage
}

func (v CustomConfigValue) configValue() (string, any) { return v.Type, v.Value }

type SetSessionConfigOptionRequest struct {
	SessionID SessionID       `json:"sessionId"`
	ConfigID  ConfigID        `json:"configId"`
	Value     ConfigValue     `json:"-"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

func (v SetSessionConfigOptionRequest) MarshalJSON() ([]byte, error) {
	if v.Value == nil {
		return nil, errors.New("missing config value")
	}
	kind, value := v.Value.configValue()
	type fields SetSessionConfigOptionRequest
	return json.Marshal(struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
		fields
	}{kind, value, fields(v)})
}

func (v *SetSessionConfigOptionRequest) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	if err := obj.require("sessionId", "configId"); err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	raw, present := obj["value"]
	if !present {
		return errors.New("missing config value")
	}
	type fields SetSessionConfigOptionRequest
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch kind {
	case "id":
		if err := obj.require("value"); err != nil {
			return err
		}
		var selection SelectConfigValue
		if err := json.Unmarshal(raw, &selection); err != nil {
			return err
		}
		value.Value = selection
	case "boolean":
		if err := obj.require("value"); err != nil {
			return err
		}
		var toggle BooleanConfigValue
		if err := json.Unmarshal(raw, &toggle); err != nil {
			return err
		}
		value.Value = toggle
	default:
		value.Value = CustomConfigValue{Type: kind, Value: raw}
	}
	*v = SetSessionConfigOptionRequest(value)
	return nil
}

type SetSessionConfigOptionResponse struct {
	ConfigOptions []SessionConfigOption `json:"configOptions"`
	Meta          json.RawMessage       `json:"_meta,omitempty"`
}

type SessionConfigOption struct {
	Type        string                `json:"type"`
	ConfigID    ConfigID              `json:"configId"`
	Name        string                `json:"name"`
	Description *string               `json:"description,omitempty"`
	Category    *string               `json:"category,omitempty"`
	Select      *SessionConfigSelect  `json:"-"`
	Boolean     *SessionConfigBoolean `json:"-"`
	Meta        json.RawMessage       `json:"_meta,omitempty"`
	Raw         json.RawMessage       `json:"-"`
}

type SessionConfigSelect struct {
	CurrentValue string              `json:"currentValue"`
	Options      ConfigSelectOptions `json:"options"`
}

type SessionConfigBoolean struct {
	CurrentValue bool `json:"currentValue"`
}

func (o *SessionConfigOption) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	if err := obj.require("configId", "name"); err != nil {
		return err
	}
	type fields SessionConfigOption
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch kind {
	case "select":
		selection, err := decodeObject[SessionConfigSelect](data, "currentValue", "options")
		if err != nil {
			return err
		}
		value.Select = &selection
	case "boolean":
		toggle, err := decodeObject[SessionConfigBoolean](data, "currentValue")
		if err != nil {
			return err
		}
		value.Boolean = &toggle
	default:
		value.Raw = bytes.Clone(data)
	}
	*o = SessionConfigOption(value)
	return nil
}

func (o SessionConfigOption) MarshalJSON() ([]byte, error) {
	type fields SessionConfigOption
	switch o.Type {
	case "select":
		if o.Select == nil {
			return nil, errors.New("missing select option fields")
		}
		return json.Marshal(struct {
			fields
			*SessionConfigSelect
		}{fields(o), o.Select})
	case "boolean":
		if o.Boolean == nil {
			return nil, errors.New("missing boolean option fields")
		}
		return json.Marshal(struct {
			fields
			*SessionConfigBoolean
		}{fields(o), o.Boolean})
	default:
		return marshalRaw(o.Raw, fields(o))
	}
}

// ConfigSelectOptions contains either a flat list of choices or a list of
// named groups of choices.
type ConfigSelectOptions struct {
	Choices []ConfigSelectOption
	Groups  []ConfigSelectGroup
}

type ConfigSelectOption struct {
	Value       string          `json:"value"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

type ConfigSelectGroup struct {
	GroupID string               `json:"groupId"`
	Name    string               `json:"name"`
	Options []ConfigSelectOption `json:"options"`
	Meta    json.RawMessage      `json:"_meta,omitempty"`
}

func (o ConfigSelectOptions) MarshalJSON() ([]byte, error) {
	switch {
	case len(o.Choices) > 0 && len(o.Groups) == 0:
		return json.Marshal(o.Choices)
	case len(o.Groups) > 0 && len(o.Choices) == 0:
		return json.Marshal(o.Groups)
	default:
		return nil, errors.New("expected choices or groups of choices")
	}
}

func (o *ConfigSelectOptions) UnmarshalJSON(data []byte) error {
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	if len(items) == 0 {
		return errors.New("expected at least one choice or group")
	}
	choices := make([]ConfigSelectOption, 0, len(items))
	for _, raw := range items {
		choice, err := decodeObject[ConfigSelectOption](raw, "value", "name")
		if err != nil {
			break
		}
		choices = append(choices, choice)
	}
	if len(choices) == len(items) {
		*o = ConfigSelectOptions{Choices: choices}
		return nil
	}
	groups := make([]ConfigSelectGroup, 0, len(items))
	for _, raw := range items {
		group, err := decodeObject[ConfigSelectGroup](raw, "groupId", "name", "options")
		if err != nil {
			return err
		}
		if len(group.Options) == 0 {
			return errors.New("expected at least one choice in the group")
		}
		groups = append(groups, group)
	}
	*o = ConfigSelectOptions{Groups: groups}
	return nil
}
