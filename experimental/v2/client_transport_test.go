package v2_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/coder/acp-go-sdk/experimental/v2"
)

func TestClient_NotificationsDone(t *testing.T) {
	fixtures := loadWireFixtures(t)
	chunkEntered := make(chan struct{})
	releaseChunk := make(chan struct{}, 1)
	events := make(chan v2.UpdateSessionNotification, 2)
	client, peer := newScriptedPeer(t, v2.ClientHandlers{
		SessionUpdate: func(ctx context.Context, note v2.UpdateSessionNotification) error {
			if note.Update.Type() == v2.UpdateAgentMessageChunk {
				close(chunkEntered)
				select {
				case <-releaseChunk:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			events <- note
			return nil
		},
	})
	t.Cleanup(func() { close(releaseChunk) })
	initializeClient(t, peer, client)
	req := decodeFixture[v2.PromptRequest](t, fixtures["prompt_request"])
	pending := startCall(func() (v2.PromptResponse, error) { return client.Prompt(peer.ctx, req) })
	frame := peer.request(t, "session/prompt", fixtures["prompt_request"])
	peer.reply(t, frame.ID, fixtures["prompt_response"])
	if ack := awaitCall(t, peer, pending); ack.MessageID != "user-1" {
		t.Fatalf("acknowledged message ID = %q", ack.MessageID)
	}

	peer.update(t, req.SessionID, fixtures["agent_message_chunk"])
	waitClosed(t, peer.ctx, chunkEntered, "chunk handler")
	peer.update(t, req.SessionID, fixtures["state_idle"])
	if err := peer.agentOut.Close(); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, peer.ctx, client.Done(), "Done")
	select {
	case <-client.NotificationsDone():
		t.Fatal("notification drain ended while a handler was blocked")
	default:
	}

	releaseChunk <- struct{}{}
	waitClosed(t, peer.ctx, client.NotificationsDone(), "NotificationsDone")
	if len(events) != 2 {
		t.Fatalf("NotificationsDone closed after %d events, want 2", len(events))
	}
	if note := <-events; note.Update.Type() != v2.UpdateAgentMessageChunk {
		t.Fatalf("first notification = %q", note.Update.Type())
	}
	assertState(t, <-events, v2.StateIdle, v2.StopReasonEndTurn)
}

func TestClient_StrictLogRedaction(t *testing.T) {
	var logs logBuffer
	oldLogger := slog.Default()
	oldOutput, oldFlags := log.Writer(), log.Flags()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(oldLogger)
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
	})

	client, peer := newScriptedPeer(t, v2.ClientHandlers{})
	initializeClient(t, peer, client)
	const secret = "fixture-secret"
	if _, err := io.WriteString(peer.agentOut, `{"params":{"token":"`+secret+`"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	for _, frame := range []string{
		`{"jsonrpc":"2.0","params":{"token":"` + secret + `"}}`,
		`{"jsonrpc":"2.0","id":{"token":"` + secret + `"},"result":{}}`,
		`{"jsonrpc":"2.0","id":{"token":"` + secret + `"},"method":"unknown/request"}`,
		`{"jsonrpc":"2.0","method":"$/cancel_request","params":"` + secret + `"}`,
		`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":{"token":"` + secret + `"}}}`,
		`{"jsonrpc":"2.0","method":"` + secret + `"}`,
	} {
		peer.write(t, []byte(frame))
	}
	if err := peer.agentOut.Close(); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, peer.ctx, client.NotificationsDone(), "NotificationsDone")

	got := logs.String()
	for _, message := range []string{
		"failed to parse incoming message",
		"received message with neither id nor method",
		"failed to canonicalize response id",
		"failed to canonicalize inbound request id",
		"failed to parse $/cancel_request params",
		"failed to canonicalize $/cancel_request requestId",
		"failed to handle notification",
	} {
		if !strings.Contains(got, message) {
			t.Errorf("missing log diagnostic %q", message)
		}
	}
	if strings.Contains(got, secret) {
		t.Fatal("transport logs exposed peer data")
	}
}

func waitClosed(t *testing.T, ctx context.Context, ch <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", name, ctx.Err())
	}
}

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
