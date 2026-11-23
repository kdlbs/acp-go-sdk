package v2

import (
	"bytes"
	"encoding/json"
)

// ContentBlock represents prompt or display content. Raw holds variants the
// SDK leaves to the caller, including embedded resources and future types.
type ContentBlock struct {
	Type        string          `json:"type"`
	Text        *string         `json:"text,omitempty"`
	Data        *string         `json:"data,omitempty"`
	MIMEType    *string         `json:"mimeType,omitempty"`
	URI         *string         `json:"uri,omitempty"`
	Name        *string         `json:"name,omitempty"`
	Title       *string         `json:"title,omitempty"`
	Description *string         `json:"description,omitempty"`
	Size        *uint64         `json:"size,omitempty"`
	Icons       []Icon          `json:"icons,omitempty"`
	Annotations *Annotations    `json:"annotations,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

type Icon struct {
	Src      string          `json:"src"`
	MIMEType *string         `json:"mimeType,omitempty"`
	Sizes    []string        `json:"sizes,omitempty"`
	Theme    *string         `json:"theme,omitempty"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

type Annotations struct {
	Audience     []string        `json:"audience,omitempty"`
	LastModified *string         `json:"lastModified,omitempty"`
	Priority     *float64        `json:"priority,omitempty"`
	Meta         json.RawMessage `json:"_meta,omitempty"`
}

func TextBlock(text string) ContentBlock {
	return ContentBlock{Type: "text", Text: &text}
}

func (b *ContentBlock) UnmarshalJSON(data []byte) error {
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
	case "text":
		required = []string{"text"}
	case "image", "audio":
		required = []string{"data", "mimeType"}
	case "resource_link":
		required = []string{"name", "uri"}
	case "resource":
		if err := obj.require("resource"); err != nil {
			return err
		}
		fallthrough
	default:
		*b = ContentBlock{Type: kind, Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	if err := obj.require(required...); err != nil {
		return err
	}
	type fields ContentBlock
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*b = ContentBlock(value)
	return nil
}

func (b ContentBlock) MarshalJSON() ([]byte, error) {
	type fields ContentBlock
	return marshalRaw(b.Raw, fields(b))
}
