package v2_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/coder/acp-go-sdk/experimental/v2"
)

type rpcFrame struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id,omitempty"`
	Method  string            `json:"method,omitempty"`
	Params  json.RawMessage   `json:"params,omitempty"`
	Result  json.RawMessage   `json:"result,omitempty"`
	Error   *acp.RequestError `json:"error,omitempty"`
}

type scriptedPeer struct {
	ctx        context.Context
	agentOut   *io.PipeWriter
	clientIn   *io.PipeWriter
	fromClient <-chan []byte
}

func newScriptedPeer(t *testing.T, handlers v2.ClientHandlers, opts ...acp.ConnectionOption) (*v2.ClientSideConnection, *scriptedPeer) {
	t.Helper()
	clientToAgent, clientIn := io.Pipe()
	agentToClient, agentOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	closePipes := func() {
		_ = clientToAgent.Close()
		_ = clientIn.Close()
		_ = agentToClient.Close()
		_ = agentOut.Close()
	}
	stop := context.AfterFunc(ctx, closePipes)
	t.Cleanup(func() {
		stop()
		cancel()
		closePipes()
	})
	frames := make(chan []byte, 16)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(clientToAgent)
		for scanner.Scan() {
			select {
			case frames <- bytes.Clone(scanner.Bytes()):
			case <-ctx.Done():
				return
			}
		}
	}()
	client := v2.NewClientSideConnection(handlers, clientIn, agentToClient, opts...)
	return client, &scriptedPeer{ctx: ctx, fromClient: frames, agentOut: agentOut, clientIn: clientIn}
}

func (p *scriptedPeer) read(t *testing.T) rpcFrame {
	t.Helper()
	frame := decodeFixture[rpcFrame](t, await(t, p.ctx, p.fromClient))
	if frame.JSONRPC != "2.0" {
		t.Fatalf("JSON-RPC version = %q", frame.JSONRPC)
	}
	return frame
}

func (p *scriptedPeer) request(t *testing.T, method string, params json.RawMessage) rpcFrame {
	t.Helper()
	frame := p.read(t)
	if frame.Method != method || len(frame.ID) == 0 {
		t.Fatalf("received %q with ID %s, want request %q", frame.Method, frame.ID, method)
	}
	if params != nil {
		assertWireJSON(t, frame.Params, params)
	}
	return frame
}

func (p *scriptedPeer) write(t *testing.T, raw []byte) {
	t.Helper()
	var line bytes.Buffer
	if err := json.Compact(&line, raw); err != nil {
		t.Fatalf("invalid test frame %s: %v", raw, err)
	}
	line.WriteByte('\n')
	if _, err := p.agentOut.Write(line.Bytes()); err != nil {
		t.Fatalf("write peer frame %s: %v", raw, err)
	}
}

func (p *scriptedPeer) send(t *testing.T, frame rpcFrame) {
	t.Helper()
	frame.JSONRPC = "2.0"
	p.write(t, marshalFixture(t, frame))
}

func (p *scriptedPeer) reply(t *testing.T, id, result json.RawMessage) {
	t.Helper()
	p.send(t, rpcFrame{ID: id, Result: result})
}

func (p *scriptedPeer) update(t *testing.T, sessionID v2.SessionID, update json.RawMessage) {
	t.Helper()
	params := struct {
		SessionID v2.SessionID    `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}{sessionID, update}
	p.send(t, rpcFrame{Method: "session/update", Params: marshalFixture(t, params)})
}

func (p *scriptedPeer) requireReply(t *testing.T, id json.RawMessage) rpcFrame {
	t.Helper()
	frame := p.read(t)
	if frame.Method != "" || !bytes.Equal(frame.ID, id) {
		t.Fatalf("received method %q with ID %s, want response for ID %s", frame.Method, frame.ID, id)
	}
	return frame
}

func (p *scriptedPeer) requireErrorReply(t *testing.T, id json.RawMessage, code int) *acp.RequestError {
	t.Helper()
	frame := p.requireReply(t, id)
	if frame.Error == nil || frame.Error.Code != code || frame.Result != nil {
		t.Fatalf("reply for %s = result %s, error %v, want error %d", id, frame.Result, frame.Error, code)
	}
	return frame.Error
}

type callResult[T any] struct {
	value T
	err   error
}

func startCall[T any](fn func() (T, error)) <-chan callResult[T] {
	result := make(chan callResult[T], 1)
	go func() {
		value, err := fn()
		result <- callResult[T]{value, err}
	}()
	return result
}

func await[T any](t *testing.T, ctx context.Context, result <-chan T) T {
	t.Helper()
	select {
	case value, ok := <-result:
		if !ok {
			t.Fatal("peer closed before the expected frame")
		}
		return value
	case <-ctx.Done():
		t.Fatalf("waiting for peer: %v", ctx.Err())
		var zero T
		return zero
	}
}

func awaitCall[T any](t *testing.T, p *scriptedPeer, result <-chan callResult[T]) T {
	t.Helper()
	got := await(t, p.ctx, result)
	if got.err != nil {
		t.Fatal(got.err)
	}
	return got.value
}

func initializeClient(t *testing.T, p *scriptedPeer, client *v2.ClientSideConnection) {
	t.Helper()
	request := v2.InitializeRequest{Info: v2.Implementation{Name: "fixture-client", Version: "1.0"}}
	pending := startCall(func() (v2.InitializeResponse, error) { return client.Initialize(p.ctx, request) })
	frame := p.request(t, "initialize", json.RawMessage(`{"protocolVersion":2,"info":{"name":"fixture-client","version":"1.0"}}`))
	p.reply(t, frame.ID, json.RawMessage(`{"protocolVersion":2,"info":{"name":"fixture-agent","version":"1.0"}}`))
	awaitCall(t, p, pending)
}
