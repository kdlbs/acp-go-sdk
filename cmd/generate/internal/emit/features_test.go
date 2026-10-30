package emit

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/coder/acp-go-sdk/cmd/generate/internal/load"
)

func generateCurrentSchema(t *testing.T) string {
	t.Helper()
	directory := filepath.Join("..", "..", "..", "..", "schema")
	meta, err := load.ReadMeta(directory)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := load.ReadSchema(directory)
	if err != nil {
		t.Fatal(err)
	}
	unstableMeta, _, err := load.ReadMetaUnstable(directory)
	if err != nil {
		t.Fatal(err)
	}
	unstableSchema, _, err := load.ReadSchemaUnstable(directory)
	if err != nil {
		t.Fatal(err)
	}
	meta, schema, err = load.MergeStableAndUnstable(meta, schema, unstableMeta, unstableSchema)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	for _, generate := range []func(string, *load.Schema, *load.Meta) error{WriteTypesJen, WriteHelpersJen, WriteDispatchJen} {
		if err := generate(out, schema, meta); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestGenerator_CurrentSchema(t *testing.T) {
	out := generateCurrentSchema(t)
	packages, err := parser.ParseDir(token.NewFileSet(), out, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declarations := make(map[string]bool)
	for _, file := range packages["acp"].Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if node.Recv == nil {
					if declarations[node.Name.Name] {
						t.Errorf("duplicate function %s", node.Name.Name)
					}
					declarations[node.Name.Name] = true
				}
			case *ast.StructType:
				fields := make(map[string]bool)
				for _, field := range node.Fields.List {
					for _, name := range field.Names {
						if fields[name.Name] {
							t.Errorf("duplicate struct field %s", name.Name)
						}
						fields[name.Name] = true
					}
				}
			}
			return true
		})
	}
}

func TestGenerator_ForkCompatibility(t *testing.T) {
	out := generateCurrentSchema(t)
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(out, "types_gen.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"NewSessionResponse":          false,
		"LoadSessionResponse":         false,
		"ResumeSessionResponse":       false,
		"UnstableForkSessionResponse": false,
	}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if _, needed := want[spec.Name.Name]; !needed {
			return true
		}
		structure, ok := spec.Type.(*ast.StructType)
		if !ok {
			t.Errorf("%s is not a struct", spec.Name.Name)
			return false
		}
		for _, field := range structure.Fields.List {
			if len(field.Names) != 1 || field.Names[0].Name != "LegacyModels" {
				continue
			}
			pointer, ok := field.Type.(*ast.StarExpr)
			if !ok {
				t.Errorf("%s.LegacyModels is not a pointer", spec.Name.Name)
				continue
			}
			name, ok := pointer.X.(*ast.Ident)
			if !ok || name.Name != "LegacyModels" || field.Tag == nil {
				t.Errorf("%s.LegacyModels has the wrong type or tag", spec.Name.Name)
				continue
			}
			tag, err := strconv.Unquote(field.Tag.Value)
			if err != nil || reflect.StructTag(tag).Get("json") != "models,omitempty" {
				t.Errorf("%s.LegacyModels lost its wire name", spec.Name.Name)
				continue
			}
			want[spec.Name.Name] = true
		}
		return false
	})
	for name, found := range want {
		if !found {
			t.Errorf("%s lost the legacy models shim during generation", name)
		}
	}

	client, err := parser.ParseFile(token.NewFileSet(), filepath.Join(out, "client_gen.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, declaration := range client.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "isKnownMethod" || function.Recv == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if clause, ok := node.(*ast.CaseClause); ok {
				for _, value := range clause.List {
					if name, ok := value.(*ast.Ident); ok {
						known[name.Name] = true
					}
				}
			}
			return true
		})
	}
	for _, method := range []string{"ClientMethodSessionUpdate", "ClientMethodSessionRequestPermission", "ClientMethodMcpMessage"} {
		if !known[method] {
			t.Errorf("generated vendor-request guard does not recognize %s", method)
		}
	}
}

func TestHelpers_ReservedParameterNames(t *testing.T) {
	schema := &load.Schema{Defs: map[string]*load.Definition{
		"Value": {AnyOf: []*load.Definition{{
			Title: "Other", Type: "object",
			Properties: map[string]*load.Definition{"type": {Type: "string"}, "error": {Type: "string"}},
			Required:   []string{"type", "error"},
		}}},
	}}
	if err := WriteHelpersJen(t.TempDir(), schema, &load.Meta{}); err != nil {
		t.Fatal(err)
	}
}

func TestObjectUnion_OptionalDiscriminator(t *testing.T) {
	def := &load.Definition{
		Type:       "object",
		Properties: map[string]*load.Definition{"stopReason": {Type: []any{"string", "null"}}},
		AnyOf: []*load.Definition{
			{Type: "object", Properties: map[string]*load.Definition{"stopReason": {Type: "string", Const: "end_turn"}}, Required: []string{"stopReason"}},
			{Type: "object", Properties: map[string]*load.Definition{"stopReason": {Type: "string", Const: "error"}, "error": {Type: "object"}}, Required: []string{"stopReason"}},
			{Type: "object", Properties: map[string]*load.Definition{"stopReason": {Type: "string"}}, AdditionalProperties: json.RawMessage("true")},
		},
	}
	flat := flattenOptionalObjectUnion(&load.Schema{}, def)
	if len(flat.AnyOf) != 0 || flat.Properties["error"] == nil || !preservesUnevaluatedProperties(flat) {
		t.Fatalf("incomplete optional union: %+v", flat)
	}
	if len(def.AnyOf) != 3 || def.Properties["error"] != nil {
		t.Fatal("input schema was modified")
	}
}
