package acp

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestClientAllowedExtensionNotifications(t *testing.T) {
	const vendorMethod = "github.com/copilot/sessionEvent"

	t.Run("exact vendor notification reaches opted-in handler", func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()

		got := make(chan string, 1)
		_ = NewClientSideConnection(&clientFuncs{
			HandleExtensionNotificationFunc: func(_ context.Context, method string, _ json.RawMessage) error {
				got <- method
				return nil
			},
		}, c2aW, a2cR, WithAllowedExtensionNotifications(vendorMethod))

		ag := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := ag.conn.SendNotification(ctx, vendorMethod, map[string]any{"msg": "hi"}); err != nil {
			t.Fatalf("SendNotification: %v", err)
		}
		if _, err := ag.ReadTextFile(ctx, ReadTextFileRequest{Path: "/tmp/test.txt", SessionId: "test-session"}); err != nil {
			t.Fatalf("ReadTextFile: %v", err)
		}

		select {
		case method := <-got:
			if method != vendorMethod {
				t.Fatalf("unexpected method: %q", method)
			}
		case <-time.After(time.Second):
			t.Fatal("allowed vendor notification did not reach the handler")
		}
	})

	t.Run("unlisted vendor notification is still ignored", func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()

		handlerCalled := make(chan struct{}, 1)
		_ = NewClientSideConnection(&clientFuncs{
			HandleExtensionNotificationFunc: func(context.Context, string, json.RawMessage) error {
				handlerCalled <- struct{}{}
				return nil
			},
		}, c2aW, a2cR, WithAllowedExtensionNotifications(vendorMethod))

		ag := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := ag.conn.SendNotification(ctx, "github.com/copilot/otherEvent", map[string]any{"msg": "hi"}); err != nil {
			t.Fatalf("SendNotification: %v", err)
		}
		if _, err := ag.ReadTextFile(ctx, ReadTextFileRequest{Path: "/tmp/test.txt", SessionId: "test-session"}); err != nil {
			t.Fatalf("ReadTextFile: %v", err)
		}

		select {
		case <-handlerCalled:
			t.Fatal("unlisted vendor notification unexpectedly reached the handler")
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("matching is exact and case-sensitive", func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()

		handlerCalled := make(chan struct{}, 1)
		_ = NewClientSideConnection(&clientFuncs{
			HandleExtensionNotificationFunc: func(context.Context, string, json.RawMessage) error {
				handlerCalled <- struct{}{}
				return nil
			},
		}, c2aW, a2cR, WithAllowedExtensionNotifications(vendorMethod))

		ag := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := ag.conn.SendNotification(ctx, "github.com/copilot/sessionevent", map[string]any{"msg": "hi"}); err != nil {
			t.Fatalf("SendNotification: %v", err)
		}
		if _, err := ag.ReadTextFile(ctx, ReadTextFileRequest{Path: "/tmp/test.txt", SessionId: "test-session"}); err != nil {
			t.Fatalf("ReadTextFile: %v", err)
		}

		select {
		case <-handlerCalled:
			t.Fatal("case-varied vendor notification unexpectedly reached the handler")
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("underscore notifications still reach the handler with an allowlist", func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()

		got := make(chan string, 1)
		_ = NewClientSideConnection(&clientFuncs{
			HandleExtensionNotificationFunc: func(_ context.Context, method string, _ json.RawMessage) error {
				got <- method
				return nil
			},
		}, c2aW, a2cR, WithAllowedExtensionNotifications(vendorMethod))

		ag := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := ag.conn.SendNotification(ctx, "_vendor.test/notice", map[string]any{"msg": "hi"}); err != nil {
			t.Fatalf("SendNotification: %v", err)
		}
		if _, err := ag.ReadTextFile(ctx, ReadTextFileRequest{Path: "/tmp/test.txt", SessionId: "test-session"}); err != nil {
			t.Fatalf("ReadTextFile: %v", err)
		}

		select {
		case method := <-got:
			if method != "_vendor.test/notice" {
				t.Fatalf("unexpected method: %q", method)
			}
		case <-time.After(time.Second):
			t.Fatal("underscore notification did not reach the handler")
		}
	})

	t.Run("allowlist does not affect request routing", func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()

		handlerCalled := make(chan struct{}, 1)
		_ = NewClientSideConnection(&clientFuncs{
			HandleExtensionNotificationFunc: func(context.Context, string, json.RawMessage) error {
				handlerCalled <- struct{}{}
				return nil
			},
		}, c2aW, a2cR, WithAllowedExtensionNotifications(vendorMethod))

		ag := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if _, err := SendRequest[json.RawMessage](ag.conn, ctx, vendorMethod, map[string]any{"msg": "hi"}); err == nil {
			t.Fatal("expected unhandled vendor request to fail")
		}

		select {
		case <-handlerCalled:
			t.Fatal("allowed vendor request unexpectedly reached the notification handler")
		default:
		}
	})
}
