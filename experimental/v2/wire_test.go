package v2_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/coder/acp-go-sdk"
	"github.com/coder/acp-go-sdk/experimental/v2"
)

func TestWire_SchemaPin(t *testing.T) {
	for file, want := range map[string]string{
		"schema.json": "4e5c50f5c3e40363437d4804f932ee2939cff25aa16e2882991430aa8102d5ad",
		"meta.json":   "5eac58a28615d399af1db8e3d636767a7e404f2df38c9668a1ba00b1989e0714",
	} {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "schema", "v2", file))
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
				t.Fatalf("SHA-256 = %s, want %s", got, want)
			}
		})
	}
}

func TestWire_Payloads(t *testing.T) {
	fixtures := loadWireFixtures(t)
	cases := []struct {
		fixture string
		value   any
	}{
		{"initialize_request", &v2.InitializeRequest{}},
		{"initialize_response", &v2.InitializeResponse{}},
		{"auth_login", &v2.LoginAuthRequest{}},
		{"auth_logout", &v2.LogoutAuthRequest{}},
		{"auth_response", &v2.LoginAuthResponse{}},
		{"auth_response", &v2.LogoutAuthResponse{}},
		{"session_new", &v2.NewSessionRequest{}},
		{"session_new_response", &v2.NewSessionResponse{}},
		{"session_resume_start", &v2.ResumeSessionRequest{}},
		{"session_resume", &v2.ResumeSessionRequest{}},
		{"session_resume_response", &v2.ResumeSessionResponse{}},
		{"session_list", &v2.ListSessionsRequest{}},
		{"session_list_response", &v2.ListSessionsResponse{}},
		{"session_close", &v2.CloseSessionRequest{}},
		{"session_close_response", &v2.CloseSessionResponse{}},
		{"session_cancel", &v2.CancelSessionNotification{}},
		{"config_set_select", &v2.SetSessionConfigOptionRequest{}},
		{"config_set_boolean", &v2.SetSessionConfigOptionRequest{}},
		{"config_set_response", &v2.SetSessionConfigOptionResponse{}},
		{"prompt_request", &v2.PromptRequest{}},
		{"prompt_response", &v2.PromptResponse{}},
		{"permission_request", &v2.RequestPermissionRequest{}},
		{"permission_response", &v2.RequestPermissionResponse{}},
		{"elicitation_form", &v2.CreateElicitationRequest{}},
		{"elicitation_url", &v2.CreateElicitationRequest{}},
		{"elicitation_response", &v2.CreateElicitationResponse{}},
		{"elicitation_complete", &v2.CompleteElicitationNotification{}},
	}
	for _, tt := range cases {
		t.Run(tt.fixture, func(t *testing.T) {
			wireRoundTrip(t, fixtures[tt.fixture], tt.value)
		})
	}

	newSession := decodeFixture[v2.NewSessionRequest](t, fixtures["session_new"])
	if len(newSession.MCPServers) != 2 || newSession.MCPServers[0].Stdio == nil || newSession.MCPServers[1].HTTP == nil {
		t.Fatalf("MCP servers = %+v", newSession.MCPServers)
	}
	replay := v2.ResumeSessionRequest{
		SessionID: "session-1", Cwd: newSession.Cwd, MCPServers: newSession.MCPServers,
		AdditionalDirectories: newSession.AdditionalDirectories, ReplayFrom: v2.ReplayFromStart(),
	}
	assertWireJSON(t, marshalFixture(t, replay), fixtures["session_resume_start"])
	assertWireJSON(t, marshalFixture(t, v2.ResumeSessionRequest{
		SessionID: replay.SessionID, Cwd: replay.Cwd,
	}), fixtures["session_resume"])

	for _, tt := range []struct {
		fixture string
		id      v2.ConfigID
		value   v2.ConfigValue
	}{
		{"config_set_select", "mode", v2.SelectConfigValue("slow")},
		{"config_set_boolean", "review", v2.BooleanConfigValue(false)},
	} {
		request := v2.SetSessionConfigOptionRequest{SessionID: "session-1", ConfigID: tt.id, Value: tt.value}
		assertWireJSON(t, marshalFixture(t, request), fixtures[tt.fixture])
	}
	result := decodeFixture[v2.NewSessionResponse](t, fixtures["session_new_response"])
	if len(result.ConfigOptions) != 2 ||
		result.ConfigOptions[0].Select == nil || result.ConfigOptions[0].Select.CurrentValue != "fast" ||
		result.ConfigOptions[1].Boolean == nil || result.ConfigOptions[1].Boolean.CurrentValue {
		t.Fatalf("config options = %+v", result.ConfigOptions)
	}
}

func TestWire_Updates(t *testing.T) {
	fixtures := loadWireFixtures(t)
	cases := []struct {
		fixture string
		kind    v2.SessionUpdateType
		value   v2.SessionUpdateValue
	}{
		{"state_running", v2.UpdateState, &v2.StateUpdate{}},
		{"state_idle", v2.UpdateState, &v2.StateUpdate{}},
		{"replay_complete", v2.UpdateReplayComplete, &v2.ReplayComplete{}},
		{"user_message", v2.UpdateUserMessage, &v2.MessageUpdate{}},
		{"user_message_chunk", v2.UpdateUserMessageChunk, &v2.MessageChunk{}},
		{"agent_message", v2.UpdateAgentMessage, &v2.MessageUpdate{}},
		{"agent_message_chunk", v2.UpdateAgentMessageChunk, &v2.MessageChunk{}},
		{"agent_thought", v2.UpdateAgentThought, &v2.MessageUpdate{}},
		{"agent_thought_chunk", v2.UpdateAgentThoughtChunk, &v2.MessageChunk{}},
		{"tool_call", v2.UpdateToolCall, &v2.ToolCallUpdate{}},
		{"tool_call_chunk", v2.UpdateToolCallContentChunk, &v2.ToolCallContentChunk{}},
		{"terminal", v2.UpdateTerminal, &v2.TerminalUpdate{}},
		{"terminal_chunk", v2.UpdateTerminalOutputChunk, &v2.TerminalOutputChunk{}},
		{"session_message", v2.UpdateSessionMessage, &v2.SessionMessage{}},
		{"session_message_chunk", v2.UpdateSessionMessageChunk, &v2.MessageChunk{}},
		{"config_update", v2.UpdateConfigOption, &v2.ConfigOptionUpdate{}},
		{"commands_update", v2.UpdateAvailableCommands, &v2.AvailableCommandsUpdate{}},
		{"session_info", v2.UpdateSessionInfo, &v2.SessionInfoUpdate{}},
		{"plan_items", v2.UpdatePlan, &v2.PlanUpdate{}},
		{"plan_markdown", v2.UpdatePlan, &v2.PlanUpdate{}},
		{"plan_removed", v2.UpdatePlanRemoved, &v2.PlanRemoved{}},
		{"usage", v2.UpdateUsage, &v2.UsageUpdate{}},
		{"notice", v2.UpdateNotice, &v2.Notice{}},
		{"compaction", v2.UpdateCompaction, &v2.CompactionUpdate{}},
		{"compaction_chunk", v2.UpdateCompactionSummaryChunk, &v2.CompactionSummaryChunk{}},
		{"future_update", "future_update", &v2.UnknownSessionUpdate{}},
	}
	for _, tt := range cases {
		t.Run(tt.fixture, func(t *testing.T) {
			var update v2.SessionUpdate
			wireRoundTrip(t, fixtures[tt.fixture], &update)
			if update.Type() != tt.kind || reflect.TypeOf(update.Value) != reflect.TypeOf(tt.value) {
				t.Fatalf("update = %s (%T), want %s (%T)", update.Type(), update.Value, tt.kind, tt.value)
			}
		})
	}
	var note v2.UpdateSessionNotification
	wireRoundTrip(t, fixtures["subagent"], &note)
	child := note.Update.Value.(*v2.SubagentUpdate)
	if note.SessionID != "session-1" || child.SessionID != "child-1" {
		t.Fatalf("parent = %q, child = %q", note.SessionID, child.SessionID)
	}

	var upsert, chunk v2.SessionUpdate
	wireRoundTrip(t, fixtures["agent_message"], &upsert)
	wireRoundTrip(t, fixtures["agent_message_chunk"], &chunk)
	message := upsert.Value.(*v2.MessageUpdate)
	part := chunk.Value.(*v2.MessageChunk)
	content, ok := message.Content.Get()
	if !ok || content == nil || len(content) != 0 || part.MessageID != message.MessageID ||
		part.Content.Text == nil || *part.Content.Text != "Fixture answer" {
		t.Fatalf("message = %+v, chunk = %+v", message, part)
	}
}

func TestWire_PatchPresence(t *testing.T) {
	cases := []struct {
		name    string
		fields  string
		content acp.Optional[[]v2.ToolCallContent]
		status  acp.Optional[string]
		meta    acp.Optional[json.RawMessage]
	}{
		{"omitted", "", acp.Optional[[]v2.ToolCallContent]{}, acp.Optional[string]{}, acp.Optional[json.RawMessage]{}},
		{"null", `,"content":null,"status":null,"_meta":null`, acp.OptionalNull[[]v2.ToolCallContent](), acp.OptionalNull[string](), acp.OptionalNull[json.RawMessage]()},
		{"empty", `,"content":[],"status":"","_meta":{}`, acp.OptionalValue([]v2.ToolCallContent{}), acp.OptionalValue(""), acp.OptionalValue(json.RawMessage(`{}`))},
		{"populated", `,"content":[{"type":"content","content":{"type":"text","text":"fixture"}}],"status":"future_status","_meta":{"n":9007199254740993}`,
			acp.OptionalValue([]v2.ToolCallContent{{Type: "content", Content: acp.Ptr(v2.TextBlock("fixture"))}}),
			acp.OptionalValue("future_status"), acp.OptionalValue(json.RawMessage(`{"n":9007199254740993}`))},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(`{"sessionUpdate":"tool_call_update","toolCallId":"call-1"` + tt.fields + `}`)
			update := decodeFixture[v2.SessionUpdate](t, raw)
			want := v2.ToolCallUpdate{ToolCallID: "call-1", Content: tt.content, Status: tt.status, Meta: tt.meta}
			if got := update.Value.(*v2.ToolCallUpdate); !reflect.DeepEqual(*got, want) {
				t.Fatalf("patch = %+v, want %+v", *got, want)
			}
			assertWireJSON(t, marshalFixture(t, v2.SessionUpdate{Value: want}), raw)
		})
	}
}

func TestWire_FutureVariants(t *testing.T) {
	fixtures := loadWireFixtures(t)
	update := decodeFixture[v2.SessionUpdate](t, fixtures["future_update"])
	if raw := update.Value.(*v2.UnknownSessionUpdate).Raw; !bytes.Equal(raw, fixtures["future_update"]) {
		t.Fatalf("future payload = %s, want %s", raw, fixtures["future_update"])
	}
	for _, raw := range []string{
		`{"sessionUpdate":"state_update","state":"future_state","unknown":9007199254740993}`,
		`{"sessionUpdate":"state_update","state":"idle","stopReason":"future_reason","error":[9007199254740993],"background":[{"type":"future_work","n":9007199254740993}]}`,
		`{"sessionUpdate":"state_update","state":"idle","stopReason":"error","error":{"code":-32603,"message":"Fixture","data":{"number":9007199254740993}}}`,
		`{"sessionUpdate":"agent_message_chunk","messageId":"msg-1","content":{"type":"future_content","text":[9007199254740993]}}`,
		`{"sessionUpdate":"tool_call_content_chunk","toolCallId":"tool-1","content":{"type":"diff","changes":[{"operation":"create","path":"/sample"}]}}`,
	} {
		var update v2.SessionUpdate
		wireRoundTrip(t, json.RawMessage(raw), &update)
	}
}

func TestWire_PermissionValidation(t *testing.T) {
	for _, raw := range []string{
		`null`,
		`{"title":"Confirm","options":[{"optionId":"once","name":"Once","kind":"allow_once"}]}`,
		`{"sessionId":"session-1","options":[{"optionId":"once","name":"Once","kind":"allow_once"}]}`,
		`{"sessionId":"session-1","title":"Confirm","options":[]}`,
		`{"sessionId":"session-1","title":"Confirm","options":[null]}`,
		`{"sessionId":"session-1","title":"Confirm","options":[{"optionId":"once","kind":null,"name":"Once"}]}`,
		`{"sessionId":"session-1","title":"Confirm","subject":{"type":"tool_call","toolCall":{}},"options":[{"optionId":"once","name":"Once","kind":"allow_once"}]}`,
		`{"sessionId":"session-1","title":"Confirm","subject":{"type":"command","command":"fixture"},"options":[{"optionId":"once","name":"Once","kind":"allow_once"}]}`,
	} {
		var request v2.RequestPermissionRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Errorf("accepted invalid permission request %s", raw)
		}
	}
}

func TestWire_ElicitationScope(t *testing.T) {
	const form = `"mode":"form","message":"Choose","requestedSchema":{"type":"object","properties":{}}`
	for _, fields := range []string{
		`"sessionId":"session-1"`,
		`"sessionId":"session-1","toolCallId":"call-1"`,
		`"requestId":"rpc-1"`,
		`"requestId":9007199254740993`,
	} {
		var request v2.CreateElicitationRequest
		wireRoundTrip(t, json.RawMessage(`{`+form+`,`+fields+`}`), &request)
	}
	for _, raw := range []string{
		`{` + form + `}`,
		`{` + form + `,"sessionId":"session-1","requestId":null}`,
		`{` + form + `,"sessionId":"session-1","requestId":1}`,
		`{` + form + `,"requestId":1,"toolCallId":null}`,
		`{` + form + `,"requestId":1,"toolCallId":"call-1"}`,
		`{` + form + `,"toolCallId":"call-1"}`,
		`{` + form + `,"requestId":null}`,
		`{` + form + `,"requestId":true}`,
		`{` + form + `,"requestId":2.5}`,
		`{` + form + `,"sessionId":null}`,
		`{"mode":"form","requestId":1,"message":"Choose"}`,
		`{"mode":"form","requestId":1,"message":"Choose","requestedSchema":{"type":"array","properties":{}}}`,
		`{"mode":"form","requestId":1,"message":"Choose","requestedSchema":{"type":"object","properties":{"a":{}}}}`,
		`{"mode":"url","requestId":1,"message":"Open","url":"https://example.test/confirm"}`,
		`{"mode":"url","requestId":1,"message":"Open","elicitationId":"elicit-1","url":null}`,
	} {
		var request v2.CreateElicitationRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Errorf("accepted invalid elicitation request %s", raw)
		}
	}
	future := json.RawMessage(`{"mode":"future","message":"Fixture","requestId":1,"url":{"serial":9007199254740993}}`)
	var request v2.CreateElicitationRequest
	wireRoundTrip(t, future, &request)
	if request.Mode != "future" || !bytes.Equal(request.Raw, future) {
		t.Fatalf("future elicitation = %+v", request)
	}
}

func loadWireFixtures(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "wire.json"))
	if err != nil {
		t.Fatal(err)
	}
	return decodeFixture[map[string]json.RawMessage](t, data)
}

func decodeFixture[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return value
}

func marshalFixture(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func wireRoundTrip(t *testing.T, raw json.RawMessage, value any) {
	t.Helper()
	if err := json.Unmarshal(raw, value); err != nil {
		t.Fatal(err)
	}
	assertWireJSON(t, marshalFixture(t, value), raw)
}

func assertWireJSON(t *testing.T, got, want []byte) {
	t.Helper()
	decode := func(raw []byte) any {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		return value
	}
	if !reflect.DeepEqual(decode(got), decode(want)) {
		t.Fatalf("wire JSON = %s\nwant %s", got, want)
	}
}
