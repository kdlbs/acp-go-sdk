package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func newOriginTestClient(t *testing.T, agent Agent) (*ClientSideConnection, io.Closer) {
	t.Helper()

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	client := NewClientSideConnection(&clientFuncs{
		SessionUpdateFunc: func(context.Context, SessionNotification) error { return nil },
	}, c2aW, a2cR)
	_ = NewAgentSideConnection(agent, a2cW, c2aR)

	closer := &originTestCloser{closers: []io.Closer{c2aR, c2aW, a2cR, a2cW}}
	t.Cleanup(func() { _ = closer.Close() })
	return client, closer
}

type originTestCloser struct{ closers []io.Closer }

func (c *originTestCloser) Close() error {
	var first error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func promptOnce(ctx context.Context, client *ClientSideConnection) error {
	_, err := client.Prompt(ctx, PromptRequest{
		SessionId: SessionId("origin-test"),
		Prompt:    []ContentBlock{TextBlock("hello")},
	})
	return err
}

func requireRemoteOrigin(t *testing.T, err error, want bool) {
	t.Helper()

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected RequestError, got %T: %v", err, err)
	}
	if got := requestErr.IsRemoteResponse(); got != want {
		t.Fatalf("IsRemoteResponse() = %v, want %v for %+v", got, want, requestErr)
	}
}

func TestRemoteRequestErrorOrigin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := newOriginTestClient(t, agentFuncs{
		PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
			return PromptResponse{}, &RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"details": "SQLiteError: database is locked"},
			}
		},
	})

	for attempt := 1; attempt <= 2; attempt++ {
		err := promptOnce(ctx, client)
		requireRemoteOrigin(t, err, true)

		wrapped := fmt.Errorf("prompt attempt %d failed: %w", attempt, err)
		requireRemoteOrigin(t, wrapped, true)
	}
}

func TestLocalRequestErrorOrigin(t *testing.T) {
	t.Run("local constructor", func(t *testing.T) {
		requireRemoteOrigin(t, NewInternalError(map[string]any{"error": "local"}), false)
		requireRemoteOrigin(t, NewRequestCancelled(nil), false)
	})

	t.Run("expired context", func(t *testing.T) {
		client, _ := newOriginTestClient(t, agentFuncs{
			PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
				return PromptResponse{StopReason: "end_turn"}, nil
			},
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		requireRemoteOrigin(t, promptOnce(ctx, client), false)
	})

	t.Run("peer disconnect", func(t *testing.T) {
		client, closer := newOriginTestClient(t, agentFuncs{
			PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
				time.Sleep(200 * time.Millisecond)
				return PromptResponse{StopReason: "end_turn"}, nil
			},
		})
		go func() {
			time.Sleep(20 * time.Millisecond)
			_ = closer.Close()
		}()
		err := promptOnce(context.Background(), client)
		if !errors.Is(err, ErrPeerDisconnected) {
			t.Fatalf("expected peer disconnect, got %v", err)
		}
		requireRemoteOrigin(t, err, false)
	})

	t.Run("send failure", func(t *testing.T) {
		c2aR, c2aW := io.Pipe()
		a2cR, a2cW := io.Pipe()
		client := NewClientSideConnection(&clientFuncs{
			SessionUpdateFunc: func(context.Context, SessionNotification) error { return nil },
		}, c2aW, a2cR)
		_ = NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
		t.Cleanup(func() {
			_ = c2aR.Close()
			_ = c2aW.Close()
			_ = a2cR.Close()
			_ = a2cW.Close()
		})
		if err := c2aW.Close(); err != nil {
			t.Fatalf("close client writer: %v", err)
		}
		requireRemoteOrigin(t, promptOnce(context.Background(), client), false)
	})
}

// newRawResponseClient serves every inbound request with the bytes produced by
// responder, which receives the parsed request id.
func newRawResponseClient(t *testing.T, responder func(id json.RawMessage) []byte) *ClientSideConnection {
	t.Helper()

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(c2aR)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			var request struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || len(request.ID) == 0 {
				continue
			}
			payload := responder(request.ID)
			if len(payload) == 0 {
				continue
			}
			if _, err := a2cW.Write(append(payload, '\n')); err != nil {
				return
			}
		}
	}()

	client := NewClientSideConnection(&clientFuncs{
		SessionUpdateFunc: func(context.Context, SessionNotification) error { return nil },
	}, c2aW, a2cR)
	t.Cleanup(func() {
		_ = c2aR.Close()
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
	})
	return client
}

func originTestResponse(id json.RawMessage, body string) []byte {
	return []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,%s}`, id, body))
}

const originTestInternalError = `"error":{"code":-32603,"message":"Internal error"}`

func TestRequestErrorOriginIgnoresUncorrelatedEnvelopes(t *testing.T) {
	t.Run("uncorrelated envelope before the real reply", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := newRawResponseClient(t, func(id json.RawMessage) []byte {
			// An envelope for an id this connection never issued must not
			// create provenance. The correlated reply follows on the next line
			// and is the one the pending request observes.
			unmatched := originTestResponse(json.RawMessage(`999999`), originTestInternalError)
			correlated := originTestResponse(id, originTestInternalError)
			return append(append(unmatched, '\n'), correlated...)
		})
		requireRemoteOrigin(t, promptOnce(ctx, client), true)
	})

	t.Run("contradictory envelope", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := newRawResponseClient(t, func(id json.RawMessage) []byte {
			return originTestResponse(id, `"result":{"stopReason":"end_turn"},"error":{"code":-32603,"message":"Internal error"}`)
		})
		requireRemoteOrigin(t, promptOnce(ctx, client), false)
	})

	t.Run("undecodable success result", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := newRawResponseClient(t, func(id json.RawMessage) []byte {
			return originTestResponse(id, `"result":{"stopReason":12}`)
		})
		requireRemoteOrigin(t, promptOnce(ctx, client), false)
	})
}

func TestRequestErrorOriginNotOnWire(t *testing.T) {
	marked := NewInternalError(map[string]any{"details": "boom"})
	marked.remoteResponse = true

	encoded, err := json.Marshal(marked)
	if err != nil {
		t.Fatalf("marshal marked error: %v", err)
	}
	if strings.Contains(strings.ToLower(string(encoded)), "remote") {
		t.Fatalf("serialized error leaked provenance: %s", encoded)
	}

	var decoded RequestError
	if err := json.Unmarshal([]byte(`{"code":-32603,"message":"Internal error","isRemoteResponse":true,"remoteResponse":true}`), &decoded); err != nil {
		t.Fatalf("decode forged envelope: %v", err)
	}
	if decoded.IsRemoteResponse() {
		t.Fatal("wire data set response provenance")
	}
}
