package v2

import (
	"bytes"
	"encoding/json"

	"github.com/coder/acp-go-sdk"
)

type ConfigOptionUpdate struct {
	ConfigOptions []SessionConfigOption `json:"configOptions"`
	Meta          json.RawMessage       `json:"_meta,omitempty"`
}

func (ConfigOptionUpdate) updateType() SessionUpdateType { return UpdateConfigOption }

type AvailableCommandsUpdate struct {
	AvailableCommands []AvailableCommand `json:"availableCommands"`
	Meta              json.RawMessage    `json:"_meta,omitempty"`
}

func (AvailableCommandsUpdate) updateType() SessionUpdateType { return UpdateAvailableCommands }

type SessionInfoUpdate struct {
	Title     acp.Optional[string]          `json:"title"`
	UpdatedAt acp.Optional[string]          `json:"updatedAt"`
	Meta      acp.Optional[json.RawMessage] `json:"_meta"`
}

func (SessionInfoUpdate) updateType() SessionUpdateType { return UpdateSessionInfo }

func (u SessionInfoUpdate) MarshalJSON() ([]byte, error) {
	type fields SessionInfoUpdate
	return marshalPatch(fields(u), map[string]bool{
		"title":     u.Title.IsSet(),
		"updatedAt": u.UpdatedAt.IsSet(),
		"_meta":     u.Meta.IsSet(),
	})
}

type PlanUpdate struct {
	Plan Plan            `json:"plan"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

func (PlanUpdate) updateType() SessionUpdateType { return UpdatePlan }

type Plan struct {
	Type    string          `json:"type"`
	PlanID  PlanID          `json:"planId"`
	Entries *[]PlanEntry    `json:"entries,omitempty"`
	Content *string         `json:"content,omitempty"`
	Path    *string         `json:"path,omitempty"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

type PlanEntry struct {
	Content  string          `json:"content"`
	Priority string          `json:"priority"`
	Status   string          `json:"status"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

func (p *Plan) UnmarshalJSON(data []byte) error {
	obj, err := readObject(data)
	if err != nil {
		return err
	}
	kind, err := obj.requiredString("type")
	if err != nil {
		return err
	}
	id, err := obj.requiredString("planId")
	if err != nil {
		return err
	}
	var required string
	switch kind {
	case "items":
		required = "entries"
	case "markdown":
		required = "content"
	default:
		*p = Plan{Type: kind, PlanID: PlanID(id), Meta: obj["_meta"], Raw: bytes.Clone(data)}
		return nil
	}
	if err := obj.require(required); err != nil {
		return err
	}
	type fields Plan
	var value fields
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = Plan(value)
	return nil
}

func (p Plan) MarshalJSON() ([]byte, error) {
	type fields Plan
	return marshalRaw(p.Raw, fields(p))
}

type PlanRemoved struct {
	PlanID PlanID          `json:"planId"`
	Meta   json.RawMessage `json:"_meta,omitempty"`
}

func (PlanRemoved) updateType() SessionUpdateType { return UpdatePlanRemoved }

type UsageUpdate struct {
	Used uint64                        `json:"used"`
	Size uint64                        `json:"size"`
	Cost acp.Optional[Cost]            `json:"cost"`
	Meta acp.Optional[json.RawMessage] `json:"_meta"`
}

type Cost struct {
	Amount   json.Number     `json:"amount"`
	Currency string          `json:"currency"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

func (UsageUpdate) updateType() SessionUpdateType { return UpdateUsage }

func (u UsageUpdate) MarshalJSON() ([]byte, error) {
	type fields UsageUpdate
	return marshalPatch(fields(u), map[string]bool{
		"cost":  u.Cost.IsSet(),
		"_meta": u.Meta.IsSet(),
	})
}

type Notice struct {
	Severity    string          `json:"severity"`
	Title       string          `json:"title"`
	Description *string         `json:"description,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

func (Notice) updateType() SessionUpdateType { return UpdateNotice }

type CompactionUpdate struct {
	CompactionID CompactionID                  `json:"compactionId"`
	Status       acp.Optional[string]          `json:"status"`
	Summary      acp.Optional[[]ContentBlock]  `json:"summary"`
	Error        acp.Optional[string]          `json:"error"`
	Meta         acp.Optional[json.RawMessage] `json:"_meta"`
}

func (CompactionUpdate) updateType() SessionUpdateType { return UpdateCompaction }

func (u CompactionUpdate) MarshalJSON() ([]byte, error) {
	type fields CompactionUpdate
	return marshalPatch(fields(u), map[string]bool{
		"status":  u.Status.IsSet(),
		"summary": u.Summary.IsSet(),
		"error":   u.Error.IsSet(),
		"_meta":   u.Meta.IsSet(),
	})
}

type CompactionSummaryChunk struct {
	CompactionID CompactionID    `json:"compactionId"`
	Content      ContentBlock    `json:"content"`
	Meta         json.RawMessage `json:"_meta,omitempty"`
}

func (CompactionSummaryChunk) updateType() SessionUpdateType { return UpdateCompactionSummaryChunk }
