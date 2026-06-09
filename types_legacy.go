package acp

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
