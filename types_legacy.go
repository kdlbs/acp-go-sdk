package acp

import "context"

// LegacyModels is the pre-v0.13.5 top-level "models" payload that some agents
// (e.g. auggie 0.29.x) still emit on session/new and session/load responses.
// Upstream removed it in v0.13.5 when model selection moved to
// SessionConfigOption (category="model"). This shim restores read-only
// parsing so consumers can fall back to the legacy surface when the new one
// is absent. Write-side helpers are intentionally not provided.
type LegacyModels struct {
	AvailableModels []LegacyModelInfo `json:"availableModels"`
	CurrentModelId  string            `json:"currentModelId"`
}

type LegacyModelInfo struct {
	ModelId     string         `json:"modelId"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

// LegacyAgentMethodSessionSetModel is the JSON-RPC method name for the
// pre-v0.13.5 unstable session model API (session/set_model). Upstream removed
// this method in v0.13.5; the constant is reintroduced here (in the legacy
// file so the generated constants file stays untouched) so consumers can
// issue the legacy call to agents that haven't migrated.
const LegacyAgentMethodSessionSetModel = "session/set_model"

// UnstableSetSessionModelRequest is the pre-v0.13.5 request for
// session/set_model. Upstream removed the unstable session model surface in
// v0.13.5 when model selection moved to SessionConfigOption(category="model"
// + session/set_config_option). This shim restores the client-side request
// type so legacy agents (e.g. auggie 0.29.x) remain reachable.
type UnstableSetSessionModelRequest struct {
	SessionId SessionId      `json:"sessionId"`
	ModelId   string         `json:"modelId"`
	Meta      map[string]any `json:"_meta,omitempty"`
}

// UnstableSetSessionModelResponse is the pre-v0.13.5 (empty) response for
// session/set_model. Restored alongside the request type.
type UnstableSetSessionModelResponse struct {
	Meta map[string]any `json:"_meta,omitempty"`
}

// UnstableSetSessionModel calls the legacy session/set_model JSON-RPC method
// on the connected agent. Returns a JSON-RPC -32601 (method not found) error
// if the agent doesn't implement the legacy surface; callers should treat
// that as the "use the new session/set_config_option(category=model) instead"
// signal.
func (c *ClientSideConnection) UnstableSetSessionModel(
	ctx context.Context,
	params UnstableSetSessionModelRequest,
) (UnstableSetSessionModelResponse, error) {
	resp, err := SendRequest[UnstableSetSessionModelResponse](c.conn, ctx, LegacyAgentMethodSessionSetModel, params)
	return resp, err
}
