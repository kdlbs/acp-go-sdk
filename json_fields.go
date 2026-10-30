package acp

import (
	"bytes"
	"encoding/json"
)

type jsonValidator interface{ Validate() error }

// MCP payloads are opaque. Preserve JSON numbers without float64 rounding.
func decodeOpaqueJSONField[T any](raw []byte, target *T) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func validateJSONValue[T any](value *T) error {
	if validator, ok := any(value).(jsonValidator); ok {
		return validator.Validate()
	}
	if validator, ok := any(*value).(jsonValidator); ok {
		return validator.Validate()
	}
	return nil
}

func decodeJSONField[T any](raw []byte, target *T, forgiving bool, fallback []byte) error {
	var value T
	err := json.Unmarshal(raw, &value)
	if err == nil && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		err = validateJSONValue(&value)
	}
	if err != nil {
		if !forgiving {
			return err
		}
		if len(fallback) > 0 {
			return json.Unmarshal(fallback, target)
		}
		return nil
	}
	*target = value
	return nil
}

func decodeJSONArray[T any](raw []byte) ([]T, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	if items == nil {
		return nil, nil
	}
	valid := make([]T, 0, len(items))
	for _, item := range items {
		if bytes.Equal(bytes.TrimSpace(item), []byte("null")) {
			continue
		}
		var value T
		if err := json.Unmarshal(item, &value); err != nil {
			continue
		}
		if err := validateJSONValue(&value); err != nil {
			continue
		}
		valid = append(valid, value)
	}
	return valid, nil
}

func decodeOptionalJSONArray[T any](raw []byte) (Optional[[]T], error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return OptionalNull[[]T](), nil
	}
	value, err := decodeJSONArray[T](raw)
	if err != nil {
		return Optional[[]T]{}, err
	}
	return OptionalValue(value), nil
}
