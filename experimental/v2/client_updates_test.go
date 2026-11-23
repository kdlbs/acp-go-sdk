package v2_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk/experimental/v2"
)

func TestClient_PromptAcknowledgement(t *testing.T) {
	fixtures := loadWireFixtures(t)
	for _, tt := range []struct {
		name         string
		runningFirst bool
	}{
		{"Codex order", true},
		{"Claude order", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			events := make(chan v2.UpdateSessionNotification, 3)
			runningEntered := make(chan struct{}, 1)
			releaseRunning := make(chan struct{}, 1)
			client, peer := newScriptedPeer(t, v2.ClientHandlers{
				SessionUpdate: func(ctx context.Context, note v2.UpdateSessionNotification) error {
					if state, ok := note.Update.Value.(*v2.StateUpdate); ok && state.State == v2.StateRunning {
						runningEntered <- struct{}{}
						select {
						case <-releaseRunning:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
					events <- note
					return nil
				},
			})
			t.Cleanup(func() { close(releaseRunning) })
			initializeClient(t, peer, client)
			req := decodeFixture[v2.PromptRequest](t, fixtures["prompt_request"])
			pending := startCall(func() (v2.PromptResponse, error) { return client.Prompt(peer.ctx, req) })
			frame := peer.request(t, "session/prompt", fixtures["prompt_request"])

			if tt.runningFirst {
				peer.update(t, req.SessionID, fixtures["state_running"])
				await(t, peer.ctx, runningEntered)
				peer.reply(t, frame.ID, fixtures["prompt_response"])
				blocked, cancel := context.WithTimeout(peer.ctx, 20*time.Millisecond)
				select {
				case got := <-pending:
					cancel()
					t.Fatalf("prompt returned while the earlier update was blocked: %+v", got)
				case <-blocked.Done():
					cancel()
				}
				releaseRunning <- struct{}{}
			} else {
				peer.reply(t, frame.ID, fixtures["prompt_response"])
				peer.update(t, req.SessionID, fixtures["state_running"])
				await(t, peer.ctx, runningEntered)
			}

			ack := awaitCall(t, peer, pending)
			if ack.MessageID != "user-1" {
				t.Fatalf("acknowledged message ID = %q", ack.MessageID)
			}
			if tt.runningFirst {
				select {
				case running := <-events:
					assertState(t, running, v2.StateRunning, "")
				default:
					t.Fatal("prompt returned before delivery of the earlier running update")
				}
			} else {
				releaseRunning <- struct{}{}
				assertState(t, await(t, peer.ctx, events), v2.StateRunning, "")
			}

			// The peer holds the tail of the turn until Prompt returns.
			peer.update(t, req.SessionID, fixtures["agent_message_chunk"])
			peer.update(t, req.SessionID, fixtures["state_idle"])
			chunk := await(t, peer.ctx, events)
			payload, ok := chunk.Update.Value.(*v2.MessageChunk)
			if !ok || chunk.Update.Type() != v2.UpdateAgentMessageChunk || payload.MessageID != "agent-1" ||
				payload.Content.Text == nil || *payload.Content.Text != "Fixture answer" {
				t.Fatalf("agent chunk = %+v", chunk)
			}
			assertState(t, await(t, peer.ctx, events), v2.StateIdle, v2.StopReasonEndTurn)
		})
	}
}

func TestClient_PromptRequiresMessageID(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"messageId":null}`, `{"messageId":1}`, `{"stopReason":"end_turn"}`} {
		t.Run(raw, func(t *testing.T) {
			client, peer := newScriptedPeer(t, v2.ClientHandlers{})
			initializeClient(t, peer, client)
			req := v2.PromptRequest{SessionID: "session-1", Prompt: []v2.ContentBlock{v2.TextBlock("fixture")}}
			pending := startCall(func() (v2.PromptResponse, error) { return client.Prompt(peer.ctx, req) })
			frame := peer.request(t, "session/prompt", nil)
			peer.reply(t, frame.ID, json.RawMessage(raw))
			if result := await(t, peer.ctx, pending); result.err == nil || isRemoteError(result.err) {
				t.Fatalf("malformed prompt response = %+v", result)
			}
		})
	}
}

func TestClient_ReplayOrdering(t *testing.T) {
	fixtures := loadWireFixtures(t)
	sequence := []string{"user_message", "agent_message", "agent_message_chunk", "future_update", "replay_complete", "state_idle"}
	events := make(chan v2.UpdateSessionNotification, len(sequence))
	client, peer := newScriptedPeer(t, v2.ClientHandlers{
		SessionUpdate: func(_ context.Context, note v2.UpdateSessionNotification) error {
			events <- note
			return nil
		},
	})
	initializeClient(t, peer, client)
	req := decodeFixture[v2.ResumeSessionRequest](t, fixtures["session_resume_start"])
	pending := startCall(func() (v2.ResumeSessionResponse, error) { return client.ResumeSession(peer.ctx, req) })
	frame := peer.request(t, "session/resume", fixtures["session_resume_start"])
	for _, update := range sequence {
		peer.update(t, req.SessionID, fixtures[update])
	}
	peer.reply(t, frame.ID, fixtures["session_resume_response"])
	response := awaitCall(t, peer, pending)
	assertWireJSON(t, marshalFixture(t, response), fixtures["session_resume_response"])
	for _, update := range sequence {
		select {
		case note := <-events:
			if note.SessionID != req.SessionID {
				t.Fatalf("replay session ID = %q", note.SessionID)
			}
			assertWireJSON(t, marshalFixture(t, note.Update), fixtures[update])
		default:
			t.Fatalf("resume returned before %s", update)
		}
	}
}

func TestClient_CancelSession(t *testing.T) {
	fixtures := loadWireFixtures(t)
	events := make(chan v2.UpdateSessionNotification, 2)
	client, peer := newScriptedPeer(t, v2.ClientHandlers{
		SessionUpdate: func(_ context.Context, note v2.UpdateSessionNotification) error {
			events <- note
			return nil
		},
	})
	initializeClient(t, peer, client)
	cancel := v2.CancelSessionNotification{SessionID: "session-1"}
	peer.update(t, cancel.SessionID, fixtures["state_running"])
	assertState(t, await(t, peer.ctx, events), v2.StateRunning, "")
	if err := client.Cancel(peer.ctx, cancel); err != nil {
		t.Fatal(err)
	}
	frame := peer.read(t)
	if frame.Method != "session/cancel" || frame.ID != nil {
		t.Fatalf("cancel frame = %+v", frame)
	}
	assertWireJSON(t, frame.Params, fixtures["session_cancel"])
	peer.update(t, cancel.SessionID, json.RawMessage(`{"sessionUpdate":"state_update","state":"idle","stopReason":"cancelled"}`))
	assertState(t, await(t, peer.ctx, events), v2.StateIdle, v2.StopReasonCancelled)
}

func assertState(t *testing.T, note v2.UpdateSessionNotification, state v2.SessionState, reason v2.StopReason) {
	t.Helper()
	update, ok := note.Update.Value.(*v2.StateUpdate)
	if !ok || update.State != state {
		t.Fatalf("state update = %+v, want state %q", note.Update.Value, state)
	}
	got, present := update.StopReason.Get()
	if got != reason || present != (reason != "") {
		t.Fatalf("state stop reason = %q (present %v), want %q", got, present, reason)
	}
}
