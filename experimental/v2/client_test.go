package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/coder/acp-go-sdk/experimental/v2"
)

func TestClient_Initialize(t *testing.T) {
	fixtures := loadWireFixtures(t)
	const info = `{"name":"fixture-agent","version":"1.0"}`
	const mismatch = `"error":{"code":-32022,"message":"Unsupported protocol version","data":`
	cases := []struct {
		name   string
		reply  string
		ok     bool
		legacy bool
		remote bool
	}{
		{name: "v2 with capabilities and auth", reply: `"result":` + string(fixtures["initialize_response"]), ok: true},
		{name: "minimal v2", reply: `"result":{"protocolVersion":2,"info":` + info + `}`, ok: true},
		{name: "minimal v1", reply: `"result":{"protocolVersion":1}`, legacy: true},
		{name: "v1 with auth", reply: `"result":{"protocolVersion":1,"agentInfo":` + info + `,"authMethods":[{"id":"login","name":"Login"}]}`, legacy: true},
		{name: "v1 envelope stamped v2", reply: `"result":` + string(fixtures["initialize_legacy_stamped_v2"]), legacy: true},
		{name: "v2 stamp with legacy capabilities only", reply: `"result":{"protocolVersion":2,"agentCapabilities":{}}`, legacy: true},
		{name: "v2 stamp with legacy info only", reply: `"result":{"protocolVersion":2,"agentInfo":` + info + `}`, legacy: true},
		{name: "unknown version", reply: `"result":{"protocolVersion":3,"info":` + info + `}`},
		{name: "missing version", reply: `"result":{"info":` + info + `}`},
		{name: "null version", reply: `"result":{"protocolVersion":null}`},
		{name: "text version", reply: `"result":{"protocolVersion":"1"}`},
		{name: "missing info", reply: `"result":{"protocolVersion":2}`},
		{name: "null info", reply: `"result":{"protocolVersion":2,"info":null}`},
		{name: "malformed info", reply: `"result":{"protocolVersion":2,"info":{"name":"bad","version":2}}`},
		{name: "missing info member", reply: `"result":{"protocolVersion":2,"info":{"name":"bad"}}`},
		{name: "malformed capabilities", reply: `"result":{"protocolVersion":2,"info":` + info + `,"capabilities":{"session":{"mcp":{"stdio":true}}}}`},
		{name: "malformed auth method", reply: `"result":{"protocolVersion":2,"info":` + info + `,"authMethods":[{"type":"agent","id":"v1","name":"Bad"}]}`},
		{name: "ambiguous v2", reply: `"result":{"protocolVersion":2,"info":` + info + `,"agentCapabilities":{}}`},
		{name: "ambiguous v1", reply: `"result":{"protocolVersion":1,"info":` + info + `}`},
		{name: "legacy envelope with null v2 discriminator", reply: `"result":{"protocolVersion":2,"capabilities":null,"agentCapabilities":{}}`},
		{name: "invalid legacy envelope", reply: `"result":{"protocolVersion":2,"agentCapabilities":{"loadSession":"true"}}`},
		{name: "legacy selection with invalid payload", reply: `"result":{"protocolVersion":1,"agentInfo":{"name":"bad"}}`},
		{name: "unrecognizable legacy envelope", reply: `"result":{"protocolVersion":2,"agentInfo":null}`},
		{name: "version error with v1", reply: mismatch + `{"requested":2,"supported":[1,3]}}`, legacy: true, remote: true},
		{name: "version error without v1", reply: mismatch + `{"requested":2,"supported":[3]}}`, remote: true},
		{name: "version error with wrong request", reply: mismatch + `{"requested":3,"supported":[1]}}`, remote: true},
		{name: "version error includes requested version", reply: mismatch + `{"requested":2,"supported":[1,2]}}`, remote: true},
		{name: "version error with malformed supported list", reply: mismatch + `{"requested":2,"supported":[1,null]}}`, remote: true},
		{name: "version error with text version", reply: mismatch + `{"requested":"2","supported":[1]}}`, remote: true},
		{name: "version error with out-of-range version", reply: mismatch + `{"requested":2,"supported":[1,65536]}}`, remote: true},
		{name: "version error with fractional request", reply: mismatch + `{"requested":2.00000000000000001,"supported":[1]}}`, remote: true},
		{name: "version error with missing data", reply: `"error":{"code":-32022,"message":"Unsupported protocol version"}`, remote: true},
		{name: "provider error", reply: `"error":{"code":-32000,"message":"Authentication required"}`, remote: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			client, peer := newScriptedPeer(t, v2.ClientHandlers{})
			req := decodeFixture[v2.InitializeRequest](t, fixtures["initialize_request"])
			pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(peer.ctx, req) })
			frame := peer.request(t, "initialize", fixtures["initialize_request"])
			peer.write(t, []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,%s}`, frame.ID, tt.reply)))
			result := await(t, peer.ctx, pending)
			if tt.ok {
				if result.err != nil || result.value.ProtocolVersion != 2 || result.value.Info.Name != "fixture-agent" {
					t.Fatalf("initialize = %+v, %v", result.value, result.err)
				}
				assertWireJSON(t, marshalFixture(t, result.value), []byte(tt.reply[len(`"result":`):]))
				return
			}
			if result.err == nil {
				t.Fatal("unexpected initialize success")
			}
			var legacy *v2.LegacyPeerError
			if errors.As(result.err, &legacy) != tt.legacy {
				t.Fatalf("LegacyPeerError = %v, want %v for %v", legacy, tt.legacy, result.err)
			}
			if isRemoteError(result.err) != tt.remote {
				t.Fatalf("remote error = %v, want %v for %v", isRemoteError(result.err), tt.remote, result.err)
			}
			_, err := client.NewSession(peer.ctx, v2.NewSessionRequest{Cwd: "/workspace"})
			if !errors.Is(err, v2.ErrNotInitialized) {
				t.Fatalf("NewSession after failed initialization = %v", err)
			}
		})
	}

	t.Run("concurrent initialization", func(t *testing.T) {
		client, peer := newScriptedPeer(t, v2.ClientHandlers{})
		req := v2.InitializeRequest{Info: v2.Implementation{Name: "fixture", Version: "1"}}
		pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(peer.ctx, req) })
		reqFrame := peer.request(t, "initialize", nil)
		if _, err := client.Initialize(peer.ctx, req); !errors.Is(err, v2.ErrInitializationInProgress) {
			t.Fatalf("concurrent initialize = %v", err)
		}
		if err := client.Cancel(peer.ctx, v2.CancelSessionNotification{SessionID: "unknown"}); !errors.Is(err, v2.ErrNotInitialized) {
			t.Fatalf("Cancel during initialize = %v", err)
		}
		peer.reply(t, reqFrame.ID, fixtures["initialize_response"])
		awaitCall(t, peer, pending)
		if _, err := client.Initialize(peer.ctx, req); !errors.Is(err, v2.ErrAlreadyInitialized) {
			t.Fatalf("repeated initialize = %v", err)
		}
	})
}

func TestClient_SessionCalls(t *testing.T) {
	fixtures := loadWireFixtures(t)
	client, peer := newScriptedPeer(t, v2.ClientHandlers{}, acp.WithMaxQueuedNotifications(4))
	expectGated(t, peer, client)
	req := v2.InitializeRequest{Info: v2.Implementation{Name: "fixture-client", Version: "1.0"}}
	pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(peer.ctx, req) })
	frame := peer.request(t, "initialize", nil)
	peer.reply(t, frame.ID, fixtures["initialize_response"])
	awaitCall(t, peer, pending)

	checkClientCall(t, peer, fixtures, "auth/login", "auth_login", "auth_response", client.LoginAuth)
	checkClientCall(t, peer, fixtures, "auth/logout", "auth_logout", "auth_response", client.LogoutAuth)
	checkClientCall(t, peer, fixtures, "session/new", "session_new", "session_new_response", client.NewSession)
	checkClientCall(t, peer, fixtures, "session/resume", "session_resume", "session_resume_response", client.ResumeSession)
	checkClientCall(t, peer, fixtures, "session/list", "session_list", "session_list_response", client.ListSessions)
	checkClientCall(t, peer, fixtures, "session/set_config_option", "config_set_select", "config_set_response", client.SetSessionConfigOption)
	checkClientCall(t, peer, fixtures, "session/set_config_option", "config_set_boolean", "config_set_response", client.SetSessionConfigOption)
	checkClientCall(t, peer, fixtures, "session/prompt", "prompt_request", "prompt_response", client.Prompt)
	checkClientCall(t, peer, fixtures, "session/close", "session_close", "session_close_response", client.CloseSession)
}

func expectGated(t *testing.T, p *scriptedPeer, client *v2.ClientSideConnection) {
	t.Helper()
	ctx := p.ctx
	_, login := client.LoginAuth(ctx, v2.LoginAuthRequest{})
	_, logout := client.LogoutAuth(ctx, v2.LogoutAuthRequest{})
	_, newSession := client.NewSession(ctx, v2.NewSessionRequest{})
	_, resume := client.ResumeSession(ctx, v2.ResumeSessionRequest{})
	_, list := client.ListSessions(ctx, v2.ListSessionsRequest{})
	_, config := client.SetSessionConfigOption(ctx, v2.SetSessionConfigOptionRequest{})
	_, prompt := client.Prompt(ctx, v2.PromptRequest{})
	_, closeSession := client.CloseSession(ctx, v2.CloseSessionRequest{})
	cancel := client.Cancel(ctx, v2.CancelSessionNotification{})
	for method, err := range map[string]error{
		"auth/login": login, "auth/logout": logout, "session/new": newSession,
		"session/resume": resume, "session/list": list, "session/set_config_option": config,
		"session/prompt": prompt, "session/close": closeSession, "session/cancel": cancel,
	} {
		if !errors.Is(err, v2.ErrNotInitialized) {
			t.Errorf("%s before initialize returned %v", method, err)
		}
	}
}

func checkClientCall[Request, Response any](
	t *testing.T, p *scriptedPeer, fixtures map[string]json.RawMessage,
	method, requestFixture, responseFixture string, fn func(context.Context, Request) (Response, error),
) {
	t.Helper()
	req := decodeFixture[Request](t, fixtures[requestFixture])
	pending := startCall(func() (Response, error) { return fn(p.ctx, req) })
	frame := p.request(t, method, fixtures[requestFixture])
	p.reply(t, frame.ID, fixtures[responseFixture])
	assertWireJSON(t, marshalFixture(t, awaitCall(t, p, pending)), fixtures[responseFixture])
}

func TestClient_ErrorProvenance(t *testing.T) {
	const versionError = `{"code":-32022,"message":"Unsupported protocol version","data":{"requested":2,"supported":[1]}}`
	req := v2.InitializeRequest{Info: v2.Implementation{Name: "fixture", Version: "1"}}
	for _, tt := range []struct {
		name string
		wire string
	}{
		{"contradictory result and error", `"jsonrpc":"2.0","result":null,"error":` + versionError},
		{"response without body", `"jsonrpc":"2.0"`},
		{"legacy result with wrong JSON-RPC version", `"jsonrpc":"1.0","result":{"protocolVersion":1}`},
		{"version error with wrong JSON-RPC version", `"jsonrpc":"1.0","error":` + versionError},
		{"version error without a message", `"jsonrpc":"2.0","error":{"code":-32022,"data":{"requested":2,"supported":[1]}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, peer := newScriptedPeer(t, v2.ClientHandlers{})
			pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(peer.ctx, req) })
			frame := peer.request(t, "initialize", nil)
			peer.write(t, []byte(fmt.Sprintf(`{"id":%s,%s}`, frame.ID, tt.wire)))
			requireLocalInitializeError(t, await(t, peer.ctx, pending).err)
		})
	}
	t.Run("uncorrelated version error", func(t *testing.T) {
		client, peer := newScriptedPeer(t, v2.ClientHandlers{})
		pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(peer.ctx, req) })
		frame := peer.request(t, "initialize", nil)
		peer.write(t, []byte(`{"jsonrpc":"2.0","id":9876,"error":`+versionError+`}`))
		peer.reply(t, frame.ID, json.RawMessage(`{"protocolVersion":2,"info":{"name":"fixture","version":"1"}}`))
		awaitCall(t, peer, pending)
	})
	t.Run("deadline", func(t *testing.T) {
		client, peer := newScriptedPeer(t, v2.ClientHandlers{})
		ctx, cancel := context.WithTimeout(peer.ctx, 150*time.Millisecond)
		defer cancel()
		pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(ctx, req) })
		peer.request(t, "initialize", nil)
		err := await(t, peer.ctx, pending).err
		requireLocalInitializeError(t, err)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("initialize deadline = %v", err)
		}
		frame := peer.read(t)
		if frame.Method != "$/cancel_request" || frame.ID != nil {
			t.Fatalf("deadline sent %+v", frame)
		}
		// The expired request does not latch initialization.
		initializeClient(t, peer, client)
	})
	t.Run("disconnect", func(t *testing.T) {
		client, peer := newScriptedPeer(t, v2.ClientHandlers{})
		pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(peer.ctx, req) })
		peer.request(t, "initialize", nil)
		if err := peer.agentOut.Close(); err != nil {
			t.Fatal(err)
		}
		err := await(t, peer.ctx, pending).err
		requireLocalInitializeError(t, err)
		if !errors.Is(err, acp.ErrPeerDisconnected) {
			t.Fatalf("initialize disconnect = %v", err)
		}
		select {
		case <-client.Done():
		case <-peer.ctx.Done():
			t.Fatal("Done did not close")
		}
	})
	t.Run("local send failure", func(t *testing.T) {
		client, peer := newScriptedPeer(t, v2.ClientHandlers{})
		if err := peer.clientIn.Close(); err != nil {
			t.Fatal(err)
		}
		_, err := client.Initialize(peer.ctx, req)
		requireLocalInitializeError(t, err)
	})
	t.Run("session error is not version selection", func(t *testing.T) {
		client, peer := newScriptedPeer(t, v2.ClientHandlers{})
		initializeClient(t, peer, client)
		pending := startCall(func() (v2.PromptResponse, error) {
			return client.Prompt(peer.ctx, v2.PromptRequest{SessionID: "session-1", Prompt: []v2.ContentBlock{v2.TextBlock("fixture")}})
		})
		frame := peer.request(t, "session/prompt", nil)
		peer.write(t, []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"error":%s}`, frame.ID, versionError)))
		err := await(t, peer.ctx, pending).err
		var legacy *v2.LegacyPeerError
		var requestErr *acp.RequestError
		if errors.As(err, &legacy) || !errors.As(err, &requestErr) || requestErr.Code != -32022 || !requestErr.IsRemoteResponse() {
			t.Fatalf("session error provenance = %v", err)
		}
	})
}

func isRemoteError(err error) bool {
	var requestErr *acp.RequestError
	return errors.As(err, &requestErr) && requestErr.IsRemoteResponse()
}

func requireLocalInitializeError(t *testing.T, err error) {
	t.Helper()
	var legacy *v2.LegacyPeerError
	if err == nil || errors.As(err, &legacy) || isRemoteError(err) {
		t.Fatalf("want local initialize error, got %v", err)
	}
}
