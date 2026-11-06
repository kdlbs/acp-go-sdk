package acp

import (
	"io"
	"log/slog"
)

// ClientSideConnection provides the client's view of the connection and implements Agent calls.
// ClientSideConnection represents the client's view of a connection to an agent.
type ClientSideConnection struct {
	conn                          *Connection
	client                        Client
	allowedExtensionNotifications map[string]struct{}
}

// NewClientSideConnection creates a new client-side connection bound to the
// provided Client implementation.
func NewClientSideConnection(client Client, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *ClientSideConnection {
	options := newConnectionOptions(opts...)
	csc := &ClientSideConnection{
		client:                        client,
		allowedExtensionNotifications: options.allowedExtensionNotifications,
	}
	csc.conn = newConnection(csc.handleWithExtensions, peerInput, peerOutput, options)
	return csc
}

// Done exposes a channel that closes when the peer disconnects.
func (c *ClientSideConnection) Done() <-chan struct{} { return c.conn.Done() }

// SetLogger directs connection diagnostics to the provided logger.
func (c *ClientSideConnection) SetLogger(l *slog.Logger) { c.conn.SetLogger(l) }
