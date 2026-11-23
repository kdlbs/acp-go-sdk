package v2

import (
	"encoding/json"
	"errors"
)

// MCPServer contains one MCP transport configuration.
type MCPServer struct {
	Stdio *MCPServerStdio
	HTTP  *MCPServerHTTP
	Other *TaggedValue
}

type MCPServerStdio struct {
	Name    string          `json:"name"`
	Command string          `json:"command"`
	Args    []string        `json:"args,omitempty"`
	Env     []EnvVariable   `json:"env,omitempty"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
}

func (s MCPServerStdio) MarshalJSON() ([]byte, error) {
	type fields MCPServerStdio
	return json.Marshal(struct {
		Type string `json:"type"`
		fields
	}{"stdio", fields(s)})
}

type MCPServerHTTP struct {
	Name    string          `json:"name"`
	URL     string          `json:"url"`
	Headers []HTTPHeader    `json:"headers,omitempty"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
}

func (s MCPServerHTTP) MarshalJSON() ([]byte, error) {
	type fields MCPServerHTTP
	return json.Marshal(struct {
		Type string `json:"type"`
		fields
	}{"http", fields(s)})
}

type EnvVariable struct {
	Name  string          `json:"name"`
	Value string          `json:"value"`
	Meta  json.RawMessage `json:"_meta,omitempty"`
}

type HTTPHeader struct {
	Name  string          `json:"name"`
	Value string          `json:"value"`
	Meta  json.RawMessage `json:"_meta,omitempty"`
}

func (s MCPServer) MarshalJSON() ([]byte, error) {
	switch {
	case s.Stdio != nil && s.HTTP == nil && s.Other == nil:
		return json.Marshal(s.Stdio)
	case s.HTTP != nil && s.Stdio == nil && s.Other == nil:
		return json.Marshal(s.HTTP)
	case s.Other != nil && s.Stdio == nil && s.HTTP == nil:
		return json.Marshal(s.Other)
	default:
		return nil, errors.New("expected one MCP server transport")
	}
}

func (s *MCPServer) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	switch kind {
	case "stdio":
		if err := obj.require("name", "command"); err != nil {
			return err
		}
		var value MCPServerStdio
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = MCPServer{Stdio: &value}
	case "http":
		if err := obj.require("name", "url"); err != nil {
			return err
		}
		var value MCPServerHTTP
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = MCPServer{HTTP: &value}
	default:
		*s = MCPServer{Other: &TaggedValue{Type: kind, Raw: append(json.RawMessage(nil), data...)}}
	}
	return nil
}
