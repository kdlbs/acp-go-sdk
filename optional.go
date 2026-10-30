package acp

import (
	"bytes"
	"encoding/json"
)

// Optional represents an omitted field, an explicit null, or a concrete value.
// Its zero value omits the field in generated compaction, subagent, and message
// patches. Use OptionalValue to replace a value and OptionalNull to clear it.
type Optional[T any] struct {
	value T
	set   bool
	null  bool
}

// OptionalValue creates a field that replaces the stored value.
func OptionalValue[T any](value T) Optional[T] {
	return Optional[T]{value: value, set: true}
}

// OptionalNull creates a field that clears the stored value.
func OptionalNull[T any]() Optional[T] {
	return Optional[T]{set: true, null: true}
}

// IsSet reports whether the patch includes the field, including an explicit null.
func (o Optional[T]) IsSet() bool { return o.set }

// IsNull reports whether the patch explicitly clears the field.
func (o Optional[T]) IsNull() bool { return o.set && o.null }

// Get returns the concrete value and whether one is present.
func (o Optional[T]) Get() (T, bool) { return o.value, o.set && !o.null }

// Validate checks a concrete value when it provides a protocol validator.
func (o Optional[T]) Validate() error {
	if !o.set || o.null {
		return nil
	}
	return validateJSONValue(&o.value)
}

// MarshalJSON encodes the value or null. Generated parents omit unset fields.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.set || o.null {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON records whether a present field contains null or a value.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*o = OptionalNull[T]()
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*o = OptionalValue(value)
	return nil
}

func marshalOptionalFields(value any, present map[string]bool) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	for key, set := range present {
		if !set {
			delete(object, key)
		}
	}
	return json.Marshal(object)
}
