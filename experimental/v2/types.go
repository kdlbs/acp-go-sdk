package v2

import "encoding/json"

type (
	SessionID          string
	MessageID          string
	ToolCallID         string
	TerminalID         string
	CompactionID       string
	PlanID             string
	AuthMethodID       string
	ConfigID           string
	PermissionOptionID string
	ElicitationID      string
)

type Implementation struct {
	Name    string          `json:"name"`
	Version string          `json:"version"`
	Title   *string         `json:"title,omitempty"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
}

func (v *Implementation) UnmarshalJSON(data []byte) error {
	type fields Implementation
	value, err := decodeObject[fields](data, "name", "version")
	if err == nil {
		*v = Implementation(value)
	}
	return err
}

// Capability indicates support when its enclosing field is present.
type Capability struct {
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type ClientCapabilities struct {
	Auth        *ClientAuthCapabilities  `json:"auth,omitempty"`
	Elicitation *ElicitationCapabilities `json:"elicitation,omitempty"`
	Meta        json.RawMessage          `json:"_meta,omitempty"`
}

type ClientAuthCapabilities struct {
	Terminal *Capability     `json:"terminal,omitempty"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

type ElicitationCapabilities struct {
	Form *Capability     `json:"form,omitempty"`
	URL  *Capability     `json:"url,omitempty"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type AgentCapabilities struct {
	Session *SessionCapabilities `json:"session,omitempty"`
	Auth    *Capability          `json:"auth,omitempty"`
	Meta    json.RawMessage      `json:"_meta,omitempty"`
}

type SessionCapabilities struct {
	Prompt                *PromptCapabilities       `json:"prompt,omitempty"`
	MCP                   *MCPCapabilities          `json:"mcp,omitempty"`
	Delete                *Capability               `json:"delete,omitempty"`
	AdditionalDirectories *Capability               `json:"additionalDirectories,omitempty"`
	SystemPrompt          *SystemPromptCapabilities `json:"systemPrompt,omitempty"`
	Meta                  json.RawMessage           `json:"_meta,omitempty"`
}

type PromptCapabilities struct {
	Image           *Capability     `json:"image,omitempty"`
	Audio           *Capability     `json:"audio,omitempty"`
	EmbeddedContext *Capability     `json:"embeddedContext,omitempty"`
	Meta            json.RawMessage `json:"_meta,omitempty"`
}

type MCPCapabilities struct {
	Stdio *Capability     `json:"stdio,omitempty"`
	HTTP  *Capability     `json:"http,omitempty"`
	Meta  json.RawMessage `json:"_meta,omitempty"`
}

type SystemPromptCapabilities struct {
	Mode *SystemPromptModeCapabilities `json:"mode,omitempty"`
	Meta json.RawMessage               `json:"_meta,omitempty"`
}

type SystemPromptModeCapabilities struct {
	Override *Capability     `json:"override,omitempty"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

type InitializeRequest struct {
	ProtocolVersion uint16              `json:"protocolVersion"`
	Info            Implementation      `json:"info"`
	Capabilities    *ClientCapabilities `json:"capabilities,omitempty"`
	Meta            json.RawMessage     `json:"_meta,omitempty"`
}

type InitializeResponse struct {
	ProtocolVersion uint16             `json:"protocolVersion"`
	Info            Implementation     `json:"info"`
	Capabilities    *AgentCapabilities `json:"capabilities,omitempty"`
	AuthMethods     []AuthMethod       `json:"authMethods,omitempty"`
	Meta            json.RawMessage    `json:"_meta,omitempty"`
}

type AuthMethod struct {
	Type        string          `json:"type"`
	MethodID    AuthMethodID    `json:"methodId"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Args        []string        `json:"args,omitempty"`
	Env         []EnvVariable   `json:"env,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

func (v *AuthMethod) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	if err := obj.require("methodId", "name"); err != nil {
		return err
	}
	type fields AuthMethod
	var value fields
	if kind == "agent" || kind == "terminal" {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		var common struct {
			MethodID AuthMethodID `json:"methodId"`
			Name     string       `json:"name"`
		}
		if err := json.Unmarshal(data, &common); err != nil {
			return err
		}
		value = fields{
			Type: kind, MethodID: common.MethodID, Name: common.Name,
			Meta: obj["_meta"], Raw: append(json.RawMessage(nil), data...),
		}
	}
	*v = AuthMethod(value)
	return nil
}

func (v AuthMethod) MarshalJSON() ([]byte, error) {
	type fields AuthMethod
	return marshalRaw(v.Raw, fields(v))
}

type LoginAuthRequest struct {
	MethodID AuthMethodID    `json:"methodId"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

type LoginAuthResponse struct {
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type LogoutAuthRequest struct {
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type LogoutAuthResponse struct {
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type NewSessionRequest struct {
	Cwd                   string          `json:"cwd"`
	AdditionalDirectories []string        `json:"additionalDirectories,omitempty"`
	MCPServers            []MCPServer     `json:"mcpServers,omitempty"`
	SystemPrompt          *SystemPrompt   `json:"systemPrompt,omitempty"`
	Meta                  json.RawMessage `json:"_meta,omitempty"`
}

type NewSessionResponse struct {
	SessionID         SessionID             `json:"sessionId"`
	ConfigOptions     []SessionConfigOption `json:"configOptions,omitempty"`
	AvailableCommands []AvailableCommand    `json:"availableCommands,omitempty"`
	Meta              json.RawMessage       `json:"_meta,omitempty"`
}

type ResumeSessionRequest struct {
	SessionID             SessionID       `json:"sessionId"`
	Cwd                   string          `json:"cwd"`
	AdditionalDirectories []string        `json:"additionalDirectories,omitempty"`
	MCPServers            []MCPServer     `json:"mcpServers,omitempty"`
	ReplayFrom            *TaggedValue    `json:"replayFrom,omitempty"`
	SystemPrompt          *SystemPrompt   `json:"systemPrompt,omitempty"`
	Meta                  json.RawMessage `json:"_meta,omitempty"`
}

type ResumeSessionResponse struct {
	ConfigOptions     []SessionConfigOption `json:"configOptions,omitempty"`
	AvailableCommands []AvailableCommand    `json:"availableCommands,omitempty"`
	Meta              json.RawMessage       `json:"_meta,omitempty"`
}

// ReplayFromStart requests replay from the start of retained history.
func ReplayFromStart() *TaggedValue { return &TaggedValue{Type: "start"} }

type SystemPrompt struct {
	Mode    string          `json:"mode"`
	Content []ContentBlock  `json:"content"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
}

type ListSessionsRequest struct {
	Cwd    *string         `json:"cwd,omitempty"`
	Cursor *string         `json:"cursor,omitempty"`
	Meta   json.RawMessage `json:"_meta,omitempty"`
}

type ListSessionsResponse struct {
	Sessions   []SessionInfo   `json:"sessions"`
	NextCursor *string         `json:"nextCursor,omitempty"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
}

type SessionInfo struct {
	SessionID             SessionID       `json:"sessionId"`
	Cwd                   string          `json:"cwd"`
	AdditionalDirectories []string        `json:"additionalDirectories,omitempty"`
	Title                 *string         `json:"title,omitempty"`
	UpdatedAt             *string         `json:"updatedAt,omitempty"`
	Meta                  json.RawMessage `json:"_meta,omitempty"`
}

type CloseSessionRequest struct {
	SessionID SessionID       `json:"sessionId"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

type CloseSessionResponse struct {
	Meta json.RawMessage `json:"_meta,omitempty"`
}

type CancelSessionNotification struct {
	SessionID SessionID       `json:"sessionId"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

type PromptRequest struct {
	SessionID      SessionID       `json:"sessionId"`
	Prompt         []ContentBlock  `json:"prompt"`
	IdempotencyKey *string         `json:"idempotencyKey,omitempty"`
	Meta           json.RawMessage `json:"_meta,omitempty"`
}

// PromptResponse acknowledges insertion of a user message. The outcome
// arrives separately through a state update.
type PromptResponse struct {
	MessageID MessageID       `json:"messageId"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

func (v *PromptResponse) UnmarshalJSON(data []byte) error {
	type fields PromptResponse
	value, err := decodeObject[fields](data, "messageId")
	if err == nil {
		*v = PromptResponse(value)
	}
	return err
}

type AvailableCommand struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Input       *TaggedValue    `json:"input,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

func (v *AvailableCommand) UnmarshalJSON(data []byte) error {
	type fields AvailableCommand
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	if err := obj.require("name", "description"); err != nil {
		return err
	}
	if kind != "command" {
		var common struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(data, &common); err != nil {
			return err
		}
		*v = AvailableCommand{
			Type: kind, Name: common.Name, Description: common.Description,
			Meta: obj["_meta"], Raw: append(json.RawMessage(nil), data...),
		}
		return nil
	}
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = AvailableCommand(value)
	return nil
}

func (v AvailableCommand) MarshalJSON() ([]byte, error) {
	type fields AvailableCommand
	return marshalRaw(v.Raw, fields(v))
}
