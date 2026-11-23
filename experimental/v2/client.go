package v2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/coder/acp-go-sdk"
)

var (
	ErrNotInitialized           = errors.New("ACP v2 connection is not initialized")
	ErrInitializationInProgress = errors.New("ACP v2 initialization is in progress")
	ErrAlreadyInitialized       = errors.New("ACP v2 connection is already initialized")
)

// ClientSideConnection makes typed ACP v2 Preview calls over stdio.
type ClientSideConnection struct {
	conn     *acp.Connection
	handlers ClientHandlers

	mu           sync.Mutex
	initializing bool
	initialized  bool
}

// NewClientSideConnection binds handlers to the agent's stdin and stdout.
// The caller owns the streams and agent process.
func NewClientSideConnection(handlers ClientHandlers, peerInput io.Writer, peerOutput io.Reader, opts ...acp.ConnectionOption) *ClientSideConnection {
	c := &ClientSideConnection{handlers: handlers}
	options := append([]acp.ConnectionOption{acp.WithStrictResponses()}, opts...)
	c.conn = acp.NewConnection(c.handleInbound, peerInput, peerOutput, options...)
	return c
}

// Done closes when the transport stops reading.
func (c *ClientSideConnection) Done() <-chan struct{} { return c.conn.Done() }

// NotificationsDone closes after the transport stops reading and all queued
// notification handlers have returned.
func (c *ClientSideConnection) NotificationsDone() <-chan struct{} {
	return c.conn.NotificationsDone()
}

// Initialize requests ACP version 2. A zero ProtocolVersion defaults to 2.
// One successful initialization enables subsequent authentication and session
// calls. A caller can retry a failed initialization. Concurrent initialization
// calls and calls after success return errors.
func (c *ClientSideConnection) Initialize(ctx context.Context, req InitializeRequest) (response InitializeResponse, err error) {
	if req.ProtocolVersion != 0 && req.ProtocolVersion != ProtocolVersionNumber {
		return response, acp.NewInvalidParams("the Preview client requires protocolVersion 2")
	}
	req.ProtocolVersion = ProtocolVersionNumber
	if err := c.checkTransport(ctx); err != nil {
		return response, err
	}
	c.mu.Lock()
	switch {
	case c.initialized:
		c.mu.Unlock()
		return response, ErrAlreadyInitialized
	case c.initializing:
		c.mu.Unlock()
		return response, ErrInitializationInProgress
	default:
		c.initializing = true
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.initializing = false
		c.initialized = err == nil
		c.mu.Unlock()
	}()

	raw, err := acp.SendRequest[json.RawMessage](c.conn, ctx, AgentMethodInitialize, req)
	if err != nil {
		return response, classifyInitializeError(err)
	}
	return decodeInitializeResponse(raw)
}

func (c *ClientSideConnection) checkTransport(ctx context.Context) error {
	select {
	case <-c.Done():
		return acp.ErrPeerDisconnected
	default:
		return ctx.Err()
	}
}

func (c *ClientSideConnection) ready(ctx context.Context) error {
	c.mu.Lock()
	initialized := c.initialized
	c.mu.Unlock()
	if !initialized {
		return ErrNotInitialized
	}
	return c.checkTransport(ctx)
}

func call[T any](c *ClientSideConnection, ctx context.Context, method string, req any) (T, error) {
	if err := c.ready(ctx); err != nil {
		var zero T
		return zero, err
	}
	return acp.SendRequest[T](c.conn, ctx, method, req)
}

func (c *ClientSideConnection) LoginAuth(ctx context.Context, req LoginAuthRequest) (LoginAuthResponse, error) {
	return call[LoginAuthResponse](c, ctx, AgentMethodAuthLogin, req)
}

func (c *ClientSideConnection) LogoutAuth(ctx context.Context, req LogoutAuthRequest) (LogoutAuthResponse, error) {
	return call[LogoutAuthResponse](c, ctx, AgentMethodAuthLogout, req)
}

func (c *ClientSideConnection) NewSession(ctx context.Context, req NewSessionRequest) (NewSessionResponse, error) {
	return call[NewSessionResponse](c, ctx, AgentMethodSessionNew, req)
}

func (c *ClientSideConnection) ResumeSession(ctx context.Context, req ResumeSessionRequest) (ResumeSessionResponse, error) {
	return call[ResumeSessionResponse](c, ctx, AgentMethodSessionResume, req)
}

func (c *ClientSideConnection) ListSessions(ctx context.Context, req ListSessionsRequest) (ListSessionsResponse, error) {
	return call[ListSessionsResponse](c, ctx, AgentMethodSessionList, req)
}

func (c *ClientSideConnection) CloseSession(ctx context.Context, req CloseSessionRequest) (CloseSessionResponse, error) {
	return call[CloseSessionResponse](c, ctx, AgentMethodSessionClose, req)
}

func (c *ClientSideConnection) SetSessionConfigOption(ctx context.Context, req SetSessionConfigOptionRequest) (SetSessionConfigOptionResponse, error) {
	return call[SetSessionConfigOptionResponse](c, ctx, AgentMethodSessionSetConfigOption, req)
}

// Prompt returns the message ID at acceptance. Foreground outcomes arrive
// separately through the session update handler.
func (c *ClientSideConnection) Prompt(ctx context.Context, req PromptRequest) (PromptResponse, error) {
	return call[PromptResponse](c, ctx, AgentMethodSessionPrompt, req)
}

// Cancel sends session/cancel as a notification.
func (c *ClientSideConnection) Cancel(ctx context.Context, note CancelSessionNotification) error {
	if err := c.ready(ctx); err != nil {
		return err
	}
	return c.conn.SendNotification(ctx, AgentMethodSessionCancel, note)
}
