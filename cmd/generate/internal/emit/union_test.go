package emit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coder/acp-go-sdk/cmd/generate/internal/load"
)

func TestObjectAnyOfCompositionAndHelpers(t *testing.T) {
	stringDefinition := func() *load.Definition { return &load.Definition{Type: "string"} }
	ref := func(name string) *load.Definition { return &load.Definition{Ref: "#/$defs/" + name} }
	schema := &load.Schema{Defs: map[string]*load.Definition{
		"SessionScope": {
			Type:       "object",
			Properties: map[string]*load.Definition{"sessionId": stringDefinition()},
			Required:   []string{"sessionId"},
		},
		"RequestScope": {
			Type:       "object",
			Properties: map[string]*load.Definition{"requestId": stringDefinition()},
			Required:   []string{"requestId"},
		},
		"FormMode": {
			Type:       "object",
			Properties: map[string]*load.Definition{"requested": stringDefinition()},
			Required:   []string{"requested"},
			AnyOf: []*load.Definition{
				{Title: "Session", AllOf: []*load.Definition{ref("SessionScope")}},
				{Title: "Request", AllOf: []*load.Definition{ref("RequestScope")}},
			},
		},
		"Create": {
			Type:       "object",
			Properties: map[string]*load.Definition{"message": stringDefinition()},
			Required:   []string{"message"},
			AnyOf: []*load.Definition{
				{
					Type:       "object",
					Properties: map[string]*load.Definition{"mode": {Type: "string", Const: "form"}},
					Required:   []string{"mode"},
					AllOf:      []*load.Definition{ref("FormMode")},
				},
			},
		},
	}}

	alternatives := expandUnionAlternatives(schema, schema.Defs["Create"].AnyOf)
	if len(alternatives) != 2 {
		t.Fatalf("expanded alternatives = %d, want 2", len(alternatives))
	}
	for _, alternative := range alternatives {
		if alternative.def.Properties["requested"] == nil || alternative.def.Properties["mode"] == nil {
			t.Fatalf("alternative %q lost base properties: %#v", alternative.nameSuffix, alternative.def.Properties)
		}
	}

	outDir := t.TempDir()
	if err := WriteTypesJen(outDir, schema, &load.Meta{}); err != nil {
		t.Fatalf("WriteTypesJen: %v", err)
	}
	if err := WriteHelpersJen(outDir, schema, &load.Meta{}); err != nil {
		t.Fatalf("WriteHelpersJen: %v", err)
	}
	typesOutput := readGeneratedFile(t, outDir, "types_gen.go")
	for _, expected := range []string{
		"type CreateFormSession struct",
		"`json:\"message\"`",
		"`json:\"requested\"`",
		"`json:\"sessionId\"`",
		"type CreateFormRequest struct",
		"`json:\"requestId\"`",
	} {
		if !strings.Contains(typesOutput, expected) {
			t.Errorf("types output missing %q", expected)
		}
	}
	helpersOutput := readGeneratedFile(t, outDir, "helpers_gen.go")
	for _, expected := range []string{
		"func NewCreateFormSession(message string, requested string, sessionId string) Create",
		"func NewCreateFormRequest(message string, requestId string, requested string) Create",
	} {
		if !strings.Contains(helpersOutput, expected) {
			t.Errorf("helpers output missing %q", expected)
		}
	}
}

func TestOpenObjectUnionVariantGeneration(t *testing.T) {
	schema := &load.Schema{Defs: map[string]*load.Definition{
		"Create": {
			Type:          "object",
			Discriminator: &load.Discriminator{PropertyName: "mode"},
			AnyOf: []*load.Definition{
				{
					Title:                 "Other",
					Type:                  "object",
					Properties:            map[string]*load.Definition{"mode": {Type: "string"}},
					Required:              []string{"mode"},
					UnevaluatedProperties: json.RawMessage("true"),
					Not: &load.Definition{AnyOf: []*load.Definition{
						{Type: "object", Properties: map[string]*load.Definition{"mode": {Type: "string", Const: "known"}}},
					}},
				},
			},
		},
	}}
	outDir := t.TempDir()
	if err := WriteTypesJen(outDir, schema, &load.Meta{}); err != nil {
		t.Fatalf("WriteTypesJen: %v", err)
	}
	output := readGeneratedFile(t, outDir, "types_gen.go")
	for _, expected := range []string{
		"Extra map[string]json.RawMessage `json:\"-\"`",
		"func (v CreateOther) MarshalJSON()",
		"func (v *CreateOther) UnmarshalJSON",
		"case \"known\":",
		"mode %q is reserved",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("types output missing %q", expected)
		}
	}
}

func readGeneratedFile(t *testing.T, directory, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(directory, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
