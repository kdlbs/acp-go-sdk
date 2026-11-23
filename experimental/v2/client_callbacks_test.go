package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/coder/acp-go-sdk"
	"github.com/coder/acp-go-sdk/experimental/v2"
)

func TestClient_Callbacks(t *testing.T) {
	fixtures := loadWireFixtures(t)
	type permissionCall struct {
		request v2.RequestPermissionRequest
		ctx     context.Context
	}
	permissions := make(chan permissionCall, 4)
	elicits := make(chan v2.CreateElicitationRequest, 4)
	completions := make(chan v2.CompleteElicitationNotification, 2)
	updates := make(chan v2.UpdateSessionNotification, 2)
	client, peer := newScriptedPeer(t, v2.ClientHandlers{
		RequestPermission: func(ctx context.Context, req v2.RequestPermissionRequest) (v2.RequestPermissionResponse, error) {
			permissions <- permissionCall{req, ctx}
			if req.Title == "Wait for cancellation" {
				<-ctx.Done()
				return v2.RequestPermissionResponse{}, ctx.Err()
			}
			return v2.RequestPermissionResponse{Outcome: v2.SelectedPermission(req.Options[0].OptionID)}, nil
		},
		CreateElicitation: func(_ context.Context, req v2.CreateElicitationRequest) (v2.CreateElicitationResponse, error) {
			elicits <- req
			if req.Message == "Return protocol error" {
				return v2.CreateElicitationResponse{}, fmt.Errorf("fixture handler: %w", &acp.RequestError{
					Code: -32001, Message: "Declined by fixture", Data: map[string]any{"fixture": true},
				})
			}
			return v2.CreateElicitationResponse{
				Action:  v2.ElicitationAccept,
				Content: map[string]json.RawMessage{"label": json.RawMessage(`"fixture"`), "serial": json.RawMessage(`9007199254740993`)},
			}, nil
		},
		CompleteElicitation: func(_ context.Context, note v2.CompleteElicitationNotification) error {
			completions <- note
			return nil
		},
		SessionUpdate: func(_ context.Context, note v2.UpdateSessionNotification) error {
			updates <- note
			return nil
		},
	})
	initializeClient(t, peer, client)
	sendRequest := func(id, method string, params json.RawMessage) json.RawMessage {
		t.Helper()
		rawID := json.RawMessage(id)
		peer.send(t, rpcFrame{ID: rawID, Method: method, Params: params})
		return rawID
	}

	updateParams := json.RawMessage(`{"sessionId":"session-1","update":{"sessionUpdate":"replay_complete"}}`)
	id := sendRequest(`"update-request"`, "session/update", updateParams)
	peer.requireErrorReply(t, id, -32601)
	id = sendRequest(`"complete-request"`, "elicitation/complete", fixtures["elicitation_complete"])
	peer.requireErrorReply(t, id, -32601)
	peer.send(t, rpcFrame{Method: "session/request_permission", Params: fixtures["permission_request"]})
	peer.send(t, rpcFrame{Method: "elicitation/create", Params: fixtures["elicitation_form"]})
	peer.send(t, rpcFrame{Method: "session/update", Params: updateParams})
	if note := await(t, peer.ctx, updates); note.Update.Type() != v2.UpdateReplayComplete {
		t.Fatalf("update = %+v", note)
	}

	id = sendRequest(`"invalid-permission"`, "session/request_permission", json.RawMessage(`{"sessionId":"session-1","title":"Bad","options":[]}`))
	peer.requireErrorReply(t, id, -32602)
	id = sendRequest(`"invalid-elicitation"`, "elicitation/create", json.RawMessage(`{"mode":"form","message":"Bad","sessionId":"session-1","requestId":2}`))
	peer.requireErrorReply(t, id, -32602)
	id = sendRequest(`"missing-params"`, "session/request_permission", nil)
	peer.requireErrorReply(t, id, -32602)
	if len(permissions) != 0 || len(elicits) != 0 || len(completions) != 0 || len(updates) != 0 {
		t.Fatal("handler invoked for an invalid or wrong-direction message")
	}

	id = sendRequest(`"permission-1"`, "session/request_permission", fixtures["permission_request"])
	permission := await(t, peer.ctx, permissions)
	if permission.request.SessionID != "session-1" || permission.request.Subject.ToolCall.ToolCallID != "call-1" ||
		!acp.IsInboundRequest(permission.ctx) {
		t.Fatalf("permission request = %+v", permission.request)
	}
	assertWireJSON(t, peer.requireReply(t, id).Result, fixtures["permission_response"])

	// Keep a permission request open while a different request succeeds.
	waiting := decodeFixture[v2.RequestPermissionRequest](t, fixtures["permission_request"])
	waiting.Title = "Wait for cancellation"
	waitingID := sendRequest("42", "session/request_permission", marshalFixture(t, waiting))
	waitingCall := await(t, peer.ctx, permissions)
	id = sendRequest(`"elicitation-1"`, "elicitation/create", fixtures["elicitation_form"])
	elicit := await(t, peer.ctx, elicits)
	if elicit.Mode != v2.ElicitationModeForm || elicit.SessionID == nil || *elicit.SessionID != "session-1" ||
		elicit.RequestedSchema.Properties["label"].Type != "string" {
		t.Fatalf("elicitation = %+v", elicit)
	}
	assertWireJSON(t, peer.requireReply(t, id).Result, fixtures["elicitation_response"])
	if waitingCall.ctx.Err() != nil {
		t.Fatal("other request cancelled the waiting permission")
	}
	peer.send(t, rpcFrame{Method: "$/cancel_request", Params: json.RawMessage(`{"requestId":42}`)})
	peer.requireErrorReply(t, waitingID, -32800)
	if !errors.Is(context.Cause(waitingCall.ctx), context.Canceled) {
		t.Fatalf("permission handler context = %v", context.Cause(waitingCall.ctx))
	}

	url := decodeFixture[v2.CreateElicitationRequest](t, fixtures["elicitation_url"])
	url.Message = "Return protocol error"
	id = sendRequest("43", "elicitation/create", marshalFixture(t, url))
	elicit = await(t, peer.ctx, elicits)
	if elicit.Mode != v2.ElicitationModeURL || string(elicit.RequestID) != "9007199254740993" {
		t.Fatalf("URL elicitation = %+v", elicit)
	}
	rpcErr := peer.requireErrorReply(t, id, -32001)
	if rpcErr.Message != "Declined by fixture" {
		t.Fatalf("handler error = %v", rpcErr)
	}
	assertWireJSON(t, marshalFixture(t, rpcErr.Data), []byte(`{"fixture":true}`))

	peer.send(t, rpcFrame{Method: "elicitation/complete", Params: fixtures["elicitation_complete"]})
	if note := await(t, peer.ctx, completions); note.ElicitationID != "elicitation-1" {
		t.Fatalf("elicitation complete = %+v", note)
	}
}

func TestClient_CallbacksOptional(t *testing.T) {
	fixtures := loadWireFixtures(t)
	client, peer := newScriptedPeer(t, v2.ClientHandlers{})
	initializeClient(t, peer, client)
	for method, fixture := range map[string]string{
		"session/request_permission": "permission_request",
		"elicitation/create":         "elicitation_form",
		"unknown/request":            "auth_logout",
	} {
		id := json.RawMessage(`"unhandled"`)
		peer.send(t, rpcFrame{ID: id, Method: method, Params: fixtures[fixture]})
		peer.requireErrorReply(t, id, -32601)
	}
}
