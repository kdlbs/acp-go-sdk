package emit

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/coder/acp-go-sdk/cmd/generate/internal/ir"
	"github.com/coder/acp-go-sdk/cmd/generate/internal/load"
	"github.com/coder/acp-go-sdk/cmd/generate/internal/util"
)

// Optional object discriminators, such as an idle state's stopReason, can be
// represented by one struct. Keep the optional discriminator and merge the
// extra variant fields rather than constructing overlapping union alternatives.
func flattenOptionalObjectUnion(schema *load.Schema, def *load.Definition) *load.Definition {
	if def == nil || len(def.AnyOf) == 0 || len(def.Properties) == 0 {
		return def
	}
	discriminator := ""
	for _, raw := range def.AnyOf {
		if raw == nil {
			continue
		}
		for key, property := range raw.Properties {
			if property != nil && property.Const != nil && includesNull(def.Properties[key]) && !slices.Contains(def.Required, key) {
				discriminator = key
			}
		}
	}
	if discriminator == "" {
		return def
	}
	merged := *def
	merged.AnyOf = nil
	merged.Properties = make(map[string]*load.Definition, len(def.Properties))
	for key, property := range def.Properties {
		merged.Properties[key] = property
	}
	for _, raw := range def.AnyOf {
		if raw == nil {
			continue
		}
		variant := expandAllOf(schema, raw)
		for _, key := range variant.Required {
			if key != discriminator {
				return def
			}
		}
		for key, property := range variant.Properties {
			if key != discriminator {
				merged.Properties[key] = property
			}
		}
		if preservesUnevaluatedProperties(variant) {
			merged.AdditionalProperties = []byte("true")
		}
	}
	return &merged
}

// These v1 entities use omission/null/value patches. Keep their Go fields
// explicit so decoding and forwarding cannot turn a clear into an omission.
func isPatchField(name, property string) bool {
	switch strings.TrimPrefix(name, "Unstable") {
	case "CompactionUpdate", "SessionCompactionUpdate":
		return property == "summary" || property == "error" || property == "_meta"
	case "SubagentUpdate", "SessionSubagentUpdate":
		return property != "sessionId" && property != "sessionUpdate"
	case "SessionMessage", "SessionUpdateSessionMessage":
		return property != "messageId" && property != "sessionUpdate"
	default:
		return false
	}
}

func objectFieldType(name, property string, def *load.Definition) Code {
	if isPatchField(name, property) {
		alternatives := def.AnyOf
		if len(alternatives) == 0 {
			alternatives = def.OneOf
		}
		if len(alternatives) == 2 {
			for _, alternative := range alternatives {
				if alternative != nil && alternative.Type != "null" {
					def = alternative
				}
			}
		}
		return Id("Optional").Types(jenTypeFor(def))
	}
	return jenTypeForOptional(def)
}

func emitPatchMarshal(f *File, name string, properties map[string]*load.Definition) {
	var keys []string
	for key := range properties {
		if isPatchField(name, key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return
	}
	sort.Strings(keys)
	fields := Dict{}
	for _, key := range keys {
		fields[Lit(key)] = Id("v").Dot(util.ToExportedField(key)).Dot("IsSet").Call()
	}
	f.Func().Params(Id("v").Id(name)).Id("MarshalJSON").Params().Params(Index().Byte(), Error()).BlockFunc(func(g *Group) {
		g.Type().Id("Alias").Id(name)
		g.Return(Id("marshalOptionalFields").Call(Id("Alias").Call(Id("v")), Map(String()).Bool().Values(fields)))
	})
	f.Line()
}

func isOpaqueMcpField(name, key string) bool {
	base := strings.TrimPrefix(name, "Unstable")
	return (base == "MessageMcpResult" && key == "result") ||
		(base == "McpError" && key == "data") ||
		((base == "MessageMcpRequest" || base == "MessageMcpNotification") && key == "params")
}

func emitObjectUnmarshal(f *File, name string, properties map[string]*load.Definition, preserveUnknown bool) {
	needed := preserveUnknown
	fastPath := !preserveUnknown
	keys := make([]string, 0, len(properties))
	for key, property := range properties {
		keys = append(keys, key)
		needed = needed || property.DefaultOnError || property.DefaultOnNull || property.Default != nil
		fastPath = fastPath && !property.SkipInvalidItems && !property.DefaultOnNull && property.Default == nil && !isPatchField(name, key) && !isOpaqueMcpField(name, key)
	}
	if !needed {
		return
	}
	sort.Strings(keys)
	f.Func().Params(Id("v").Op("*").Id(name)).Id("UnmarshalJSON").Params(Id("data").Index().Byte()).Error().BlockFunc(func(g *Group) {
		if fastPath {
			// Avoid copying large content fields into maps when normal decoding succeeds.
			g.Type().Id("Alias").Id(name)
			g.Var().Id("quick").Id("Alias")
			g.If(Op("!").Qual("bytes", "Equal").Call(Qual("bytes", "TrimSpace").Call(Id("data")), Index().Byte().Parens(Lit("null")))).Block(
				If(Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(Id("data"), Op("&").Id("quick")), Id("err").Op("==").Nil()).Block(
					Op("*").Id("v").Op("=").Id(name).Call(Id("quick")), Return(Nil()),
				),
			)
		}
		g.Var().Id("object").Map(String()).Qual("encoding/json", "RawMessage")
		g.If(Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(Id("data"), Op("&").Id("object")), Id("err").Op("!=").Nil()).Block(Return(Id("err")))
		g.If(Id("object").Op("==").Nil()).Block(Return(Qual("errors", "New").Call(Lit("expected an object"))))
		g.Var().Id("decoded").Id(name)
		for _, key := range keys {
			property := properties[key]
			g.BlockFunc(func(h *Group) {
				h.List(Id("raw"), Id("ok")).Op(":=").Id("object").Index(Lit(key))
				defaultJSON := ""
				if property.Default != nil {
					encoded, _ := json.Marshal(property.Default)
					defaultJSON = string(encoded)
				} else if property.DefaultOnNull && ir.PrimaryType(property) == "array" {
					defaultJSON = "[]"
				}
				if defaultJSON != "" {
					condition := Op("!").Id("ok")
					if !includesNull(property) || property.DefaultOnNull {
						condition.Op("||").Qual("bytes", "Equal").Call(Qual("bytes", "TrimSpace").Call(Id("raw")), Index().Byte().Parens(Lit("null")))
					}
					h.If(condition).Block(Id("raw").Op("=").Index().Byte().Parens(Lit(defaultJSON)), Id("ok").Op("=").Lit(true))
				}
				if strings.HasSuffix(name, "McpError") && (key == "code" || key == "message") {
					h.If(Op("!").Id("ok").Op("||").Id("string").Call(Id("raw")).Op("==").Lit("null")).Block(Return(Qual("errors", "New").Call(Lit(key + " is required"))))
				}
				h.If(Id("ok")).BlockFunc(func(body *Group) {
					field := Id("decoded").Dot(util.ToExportedField(key))
					if isOpaqueMcpField(name, key) {
						body.If(Id("err").Op(":=").Id("decodeOpaqueJSONField").Call(Id("raw"), Op("&").Add(field)), Id("err").Op("!=").Nil()).Block(Return(Id("err")))
						return
					}
					if property.SkipInvalidItems && ir.PrimaryType(property) == "array" {
						function := "decodeJSONArray"
						if isPatchField(name, key) {
							function = "decodeOptionalJSONArray"
						}
						body.List(Id("value"), Id("err")).Op(":=").Id(function).Types(jenTypeFor(property.Items)).Call(Id("raw"))
						body.If(Id("err").Op("==").Nil()).Block(Id("decoded").Dot(util.ToExportedField(key)).Op("=").Id("value"))
						if !property.DefaultOnError {
							body.If(Id("err").Op("!=").Nil()).Block(Return(Id("err")))
						}
					} else {
						body.If(Id("err").Op(":=").Id("decodeJSONField").Call(Id("raw"), Op("&").Add(field), Lit(property.DefaultOnError), Index().Byte().Parens(Lit(defaultJSON))), Id("err").Op("!=").Nil()).Block(Return(Id("err")))
					}
				})
			})
		}
		if needsLegacyModelsField(name, &load.Definition{Properties: properties}) {
			g.If(List(Id("raw"), Id("ok")).Op(":=").Id("object").Index(Lit("models")), Id("ok")).Block(
				If(Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(Id("raw"), Op("&").Id("decoded").Dot("LegacyModels")), Id("err").Op("!=").Nil()).Block(Return(Id("err"))),
			)
		}
		if preserveUnknown {
			for _, key := range keys {
				g.Id("delete").Call(Id("object"), Lit(key))
			}
			g.Id("decoded").Dot("Extra").Op("=").Id("object")
		}
		g.Op("*").Id("v").Op("=").Id("decoded")
		g.Return(Nil())
	})
	f.Line()
}
