package v2

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/coder/acp-go-sdk"
)

// ClientHandlers receives callbacks from the agent. A nil handler leaves its
// method unsupported. Notification handlers run in wire order and must return
// before the connection can deliver responses that followed those notifications.
// Request handlers can run concurrently and receive cancellable contexts.
type ClientHandlers struct {
	SessionUpdate       func(context.Context, UpdateSessionNotification) error
	RequestPermission   func(context.Context, RequestPermissionRequest) (RequestPermissionResponse, error)
	CreateElicitation   func(context.Context, CreateElicitationRequest) (CreateElicitationResponse, error)
	CompleteElicitation func(context.Context, CompleteElicitationNotification) error
}

func (c *ClientSideConnection) handleInbound(ctx context.Context, method string, params json.RawMessage) (any, *acp.RequestError) {
	if acp.IsInboundRequest(ctx) {
		switch method {
		case ClientMethodSessionRequestPermission:
			return dispatchRequest(ctx, method, params, c.handlers.RequestPermission)
		case ClientMethodElicitationCreate:
			return dispatchRequest(ctx, method, params, c.handlers.CreateElicitation)
		}
	} else {
		switch method {
		case ClientMethodSessionUpdate:
			return nil, dispatchNotification(ctx, method, params, c.handlers.SessionUpdate)
		case ClientMethodElicitationComplete:
			return nil, dispatchNotification(ctx, method, params, c.handlers.CompleteElicitation)
		}
	}
	return nil, acp.NewMethodNotFound(method)
}

func dispatchRequest[Request, Response any](ctx context.Context, method string, params json.RawMessage, handler func(context.Context, Request) (Response, error)) (any, *acp.RequestError) {
	if handler == nil {
		return nil, acp.NewMethodNotFound(method)
	}
	var request Request
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, acp.NewInvalidParams(err.Error())
	}
	response, err := handler(ctx, request)
	return response, callbackError(err)
}

func dispatchNotification[Notification any](ctx context.Context, method string, params json.RawMessage, handler func(context.Context, Notification) error) *acp.RequestError {
	if handler == nil {
		return acp.NewMethodNotFound(method)
	}
	var note Notification
	if err := json.Unmarshal(params, &note); err != nil {
		return acp.NewInvalidParams(err.Error())
	}
	return callbackError(handler(ctx, note))
}

func callbackError(err error) *acp.RequestError {
	if err == nil {
		return nil
	}
	var protocolErr *acp.RequestError
	if errors.As(err, &protocolErr) && protocolErr != nil {
		return protocolErr
	}
	if errors.Is(err, context.Canceled) {
		return acp.NewRequestCancelled(err.Error())
	}
	return acp.NewInternalError(err.Error())
}
