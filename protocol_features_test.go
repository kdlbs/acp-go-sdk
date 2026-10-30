package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSessionUpdate_NewVariantsRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		selected func(SessionUpdate) bool
	}{
		{"notice", func(u SessionUpdate) bool { return u.Notice != nil }},
		{"compaction_update", func(u SessionUpdate) bool { return u.CompactionUpdate != nil }},
		{"compaction_clear", func(u SessionUpdate) bool {
			return u.CompactionUpdate != nil && u.CompactionUpdate.Summary.IsNull() && u.CompactionUpdate.Error.IsNull()
		}},
		{"compaction_summary_chunk", func(u SessionUpdate) bool { return u.CompactionSummaryChunk != nil }},
		{"subagent_update", func(u SessionUpdate) bool { return u.SubagentUpdate != nil }},
		{"subagent_clear", func(u SessionUpdate) bool {
			return u.SubagentUpdate != nil && u.SubagentUpdate.Capabilities.IsNull() && u.SubagentUpdate.State.IsNull()
		}},
		{"subagent_idle", func(u SessionUpdate) bool { return u.SubagentUpdate != nil }},
		{"session_message", func(u SessionUpdate) bool {
			return u.SessionMessage != nil && u.SessionMessage.RecipientMessageId.IsNull()
		}},
		{"session_message_chunk", func(u SessionUpdate) bool { return u.SessionMessageChunk != nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "json_golden", "session_update_"+tt.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var update SessionUpdate
			if err := json.Unmarshal(data, &update); err != nil {
				t.Fatal(err)
			}
			if !tt.selected(update) {
				t.Fatalf("wrong variant: %+v", update)
			}
			if err := update.Validate(); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(update)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, encoded, data)
		})
	}
}

func assertJSONEqual(t *testing.T, actual, expected []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(actual, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("JSON mismatch:\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestSessionUpdate_PatchConstruction(t *testing.T) {
	child := UpdateSubagent("child-1")
	child.SubagentUpdate.Title = OptionalValue("Reviewer")
	child.SubagentUpdate.Description = OptionalNull[string]()
	child.SubagentUpdate.AdditionalDirectories = OptionalValue([]string{})
	encoded, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, encoded, []byte(`{"sessionUpdate":"subagent_update","sessionId":"child-1","title":"Reviewer","description":null,"additionalDirectories":[]}`))
	var decoded SessionUpdate
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	patch := decoded.SubagentUpdate
	if patch.Cwd.IsSet() || !patch.Description.IsNull() {
		t.Fatalf("patch presence lost: %+v", patch)
	}
	if title, ok := patch.Title.Get(); !ok || title != "Reviewer" {
		t.Fatalf("title = %q, %v", title, ok)
	}
	if err := json.Unmarshal([]byte(`{"sessionUpdate":"subagent_update","sessionId":"child-1"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SubagentUpdate.Title.IsSet() {
		t.Fatal("omitted field retained a previous value")
	}

	compaction := UpdateCompaction("compact-1", CompactionStatusCompleted)
	compaction.CompactionUpdate.Summary = OptionalValue([]ContentBlock{})
	encoded, err = json.Marshal(compaction)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, encoded, []byte(`{"sessionUpdate":"compaction_update","compactionId":"compact-1","status":"completed","summary":[]}`))
}

func TestStateUpdate_UnknownAndIdle(t *testing.T) {
	for _, data := range []string{
		`{"state":"_vendor/waiting","reason":{"ticket":42},"extra":null}`,
		`{"state":"idle","stopReason":"_vendor/finished","vendorField":[1,2]}`,
		`{"state":"idle","stopReason":"error","error":{"code":-32603,"message":"failed"}}`,
		`{"state":"idle"}`,
	} {
		t.Run(data, func(t *testing.T) {
			var state StateUpdate
			if err := json.Unmarshal([]byte(data), &state); err != nil {
				t.Fatal(err)
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, encoded, []byte(data))
		})
	}
}

func TestProtocolDecoding_FieldRules(t *testing.T) {
	var state StateUpdate
	if err := json.Unmarshal([]byte(`{"state":"idle","stopReason":17}`), &state); err != nil {
		t.Fatal(err)
	}
	if state.Idle == nil || state.Idle.StopReason != nil {
		t.Fatalf("malformed optional stop reason was retained: %+v", state)
	}
	var update SessionUpdate
	if err := json.Unmarshal([]byte(`{"sessionUpdate":"compaction_update","compactionId":"c","status":"in_progress","summary":[17,null,{"type":"image","data":"abc"},{"type":"text","text":"valid"}]}`), &update); err != nil {
		t.Fatal(err)
	}
	summary, ok := update.CompactionUpdate.Summary.Get()
	if !ok || len(summary) != 1 || summary[0].Text == nil || summary[0].Text.Text != "valid" {
		t.Fatalf("invalid summary entries were retained: %+v", summary)
	}
	for _, data := range []string{`{"cwd":"/tmp"}`, `{"cwd":"/tmp","mcpServers":null}`} {
		var request NewSessionRequest
		if err := json.Unmarshal([]byte(data), &request); err != nil {
			t.Fatal(err)
		}
		if request.McpServers == nil {
			t.Fatal("default-on-null list was not initialized")
		}
	}
	var terminal CreateTerminalRequest
	if err := json.Unmarshal([]byte(`{"command":"echo","args":["ok",42]}`), &terminal); err == nil {
		t.Fatal("malformed command arguments were accepted")
	}
}

func TestSystemPrompt_LifecycleRequests(t *testing.T) {
	prompt := SystemPromptText(SystemPromptModeAppend, "Review changes before editing.")
	tests := []any{
		NewSessionRequest{Cwd: "/tmp", SystemPrompt: &prompt},
		LoadSessionRequest{Cwd: "/tmp", SessionId: "s", SystemPrompt: &prompt},
		ResumeSessionRequest{Cwd: "/tmp", SessionId: "s", SystemPrompt: &prompt},
		UnstableForkSessionRequest{Cwd: "/tmp", SessionId: "s", SystemPrompt: &UnstableSystemPrompt{Mode: UnstableSystemPromptModeAppend, Content: []UnstableContentBlock{NewUnstableContentBlockText("Review changes before editing.")}}},
	}
	for _, request := range tests {
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]json.RawMessage
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, wire["systemPrompt"], []byte(`{"mode":"append","content":[{"type":"text","text":"Review changes before editing."}]}`))
	}
}

type elicitationClient struct {
	clientFuncs
	completed bool
}

func (elicitationClient) CreateElicitation(context.Context, CreateElicitationRequest) (CreateElicitationResponse, error) {
	return NewCreateElicitationResponseDecline(), nil
}

func (c *elicitationClient) CompleteElicitation(context.Context, CompleteElicitationNotification) error {
	c.completed = true
	return nil
}

var _ ClientElicitation = (*elicitationClient)(nil)

type legacyElicitationClient struct {
	clientFuncs
	completed bool
}

func (legacyElicitationClient) UnstableCreateElicitation(context.Context, UnstableCreateElicitationRequest) (UnstableCreateElicitationResponse, error) {
	return NewUnstableCreateElicitationResponseDecline(), nil
}

func (c *legacyElicitationClient) UnstableCompleteElicitation(context.Context, UnstableCompleteElicitationNotification) error {
	c.completed = true
	return nil
}

func TestClientElicitation_OptionalDispatch(t *testing.T) {
	params := json.RawMessage(`{"mode":"form","message":"Choose","sessionId":"s","requestedSchema":{"type":"object","properties":{}}}`)
	stable, legacy := &elicitationClient{}, &legacyElicitationClient{}
	for _, client := range []Client{stable, legacy} {
		conn := &ClientSideConnection{client: client}
		response, err := conn.handle(context.Background(), ClientMethodElicitationCreate, params)
		if err != nil {
			t.Fatal(err)
		}
		if response.(CreateElicitationResponse).Decline == nil {
			t.Fatalf("wrong response: %+v", response)
		}
		if _, err := conn.handle(context.Background(), ClientMethodElicitationComplete, json.RawMessage(`{"elicitationId":"input-1"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if !stable.completed || !legacy.completed {
		t.Fatal("completion did not reach the handler")
	}
	conn := &ClientSideConnection{client: &clientFuncs{}}
	if _, err := conn.handle(context.Background(), ClientMethodElicitationCreate, params); err == nil || err.Code != -32601 {
		t.Fatalf("unsupported elicitation returned %v", err)
	}
}

func TestMcpMessage_ResponseCarrier(t *testing.T) {
	for _, data := range []string{
		`{"result":null}`,
		`{"result":{"content":[{"type":"text","text":"ok"}],"vendor":true}}`,
		`{"error":{"code":-32000,"message":"failed","data":{"retry":true},"vendor":"kept"}}`,
	} {
		var response UnstableMessageMcpResponse
		if err := json.Unmarshal([]byte(data), &response); err != nil {
			t.Fatal(err)
		}
		if err := response.Validate(); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, encoded, []byte(data))
	}
	var response UnstableMessageMcpResponse
	if err := json.Unmarshal([]byte(`{"result":null,"error":{"code":-1,"message":"ignored"}}`), &response); err != nil || response.Result == nil || response.Error != nil {
		t.Fatalf("result precedence: %+v, %v", response, err)
	}
	for _, data := range []string{`{}`, `{"error":null}`, `{"error":{}}`, `{"error":{"code":null,"message":"failed"}}`} {
		if err := json.Unmarshal([]byte(data), &response); err == nil {
			t.Errorf("invalid carrier accepted: %s", data)
		}
	}
}

func TestMcpMessage_OpaqueNumbers(t *testing.T) {
	var response UnstableMessageMcpResponse
	for _, data := range []string{
		`{"result":{"number":9007199254740993}}`,
		`{"error":{"code":-32000,"message":"failed","data":{"number":9007199254740993}}}`,
	} {
		if err := json.Unmarshal([]byte(data), &response); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(encoded, []byte("9007199254740993")) {
			t.Fatalf("MCP number was rounded: %s", encoded)
		}
	}
	var request UnstableMessageMcpRequest
	if err := json.Unmarshal([]byte(`{"serverId":"editor","requestId":"r","method":"tools/call","params":{"number":9007199254740993}}`), &request); err != nil {
		t.Fatal(err)
	}
	if value, ok := request.Params["number"].(json.Number); !ok || value.String() != "9007199254740993" {
		t.Fatalf("MCP request number was rounded: %#v", request.Params)
	}
}

type mcpClient struct {
	clientFuncs
	requests chan UnstableMessageMcpRequest
}

func (c mcpClient) UnstableMessageMcp(ctx context.Context, request UnstableMessageMcpRequest) (UnstableMessageMcpResponse, error) {
	c.requests <- request
	return NewUnstableMessageMcpResponseResult(nil), nil
}

func TestMcpMessage_RequestScopedTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	for _, closer := range []io.Closer{c2aR, c2aW, a2cR, a2cW} {
		defer closer.Close()
	}
	requests := make(chan UnstableMessageMcpRequest, 1)
	notifications := make(chan UnstableMessageMcpNotification, 1)
	client := NewClientSideConnection(&mcpClient{requests: requests}, c2aW, a2cR)
	agent := NewAgentSideConnection(agentFuncs{
		UnstableMessageMcpFunc: func(ctx context.Context, notification UnstableMessageMcpNotification) error {
			notifications <- notification
			return nil
		},
	}, a2cW, c2aR)
	response, err := agent.UnstableMessageMcp(ctx, UnstableMessageMcpRequest{ServerId: "editor", RequestId: "inner-1", Method: "tools/list"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Result == nil || response.Result.Result != nil {
		t.Fatalf("null MCP result lost: %+v", response)
	}
	select {
	case request := <-requests:
		if request.ServerId != "editor" || request.RequestId != "inner-1" || request.Method != "tools/list" {
			t.Fatalf("request identity lost: %+v", request)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := client.UnstableMessageMcp(ctx, UnstableMessageMcpNotification{ServerId: "editor", RequestId: "inner-1", Method: "notifications/progress"}); err != nil {
		t.Fatal(err)
	}
	select {
	case notification := <-notifications:
		if notification.ServerId != "editor" || notification.RequestId != "inner-1" {
			t.Fatalf("notification identity lost: %+v", notification)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
