package acp

import "context"

// Deprecated: Use CompleteElicitationNotification.
type UnstableCompleteElicitationNotification = CompleteElicitationNotification

// Deprecated: Use CreateElicitationFormSession.
type UnstableCreateElicitationFormSession = CreateElicitationFormSession

// Deprecated: Use CreateElicitationFormRequest.
type UnstableCreateElicitationFormRequest = CreateElicitationFormRequest

// Deprecated: Use CreateElicitationUrlSession.
type UnstableCreateElicitationUrlSession = CreateElicitationUrlSession

// Deprecated: Use CreateElicitationUrlRequest.
type UnstableCreateElicitationUrlRequest = CreateElicitationUrlRequest

// Deprecated: Use CreateElicitationOtherSession.
type UnstableCreateElicitationOtherSession = CreateElicitationOtherSession

// Deprecated: Use CreateElicitationOtherRequest.
type UnstableCreateElicitationOtherRequest = CreateElicitationOtherRequest

// Deprecated: Use CreateElicitationRequest.
type UnstableCreateElicitationRequest = CreateElicitationRequest

// Deprecated: Use CreateElicitationAccept.
type UnstableCreateElicitationAccept = CreateElicitationAccept

// Deprecated: Use CreateElicitationDecline.
type UnstableCreateElicitationDecline = CreateElicitationDecline

// Deprecated: Use CreateElicitationCancel.
type UnstableCreateElicitationCancel = CreateElicitationCancel

// Deprecated: Use CreateElicitationOther.
type UnstableCreateElicitationOther = CreateElicitationOther

// Deprecated: Use CreateElicitationResponse.
type UnstableCreateElicitationResponse = CreateElicitationResponse

// Deprecated: Use ElicitationAcceptAction.
type UnstableElicitationAcceptAction = ElicitationAcceptAction

// Deprecated: Use ElicitationFormModeSession.
type UnstableElicitationFormModeSession = ElicitationFormModeSession

// Deprecated: Use ElicitationFormModeRequest.
type UnstableElicitationFormModeRequest = ElicitationFormModeRequest

// Deprecated: Use ElicitationFormMode.
type UnstableElicitationFormMode = ElicitationFormMode

// Deprecated: Use ElicitationId.
type UnstableElicitationId = ElicitationId

// Deprecated: Use ElicitationRequestScope.
type UnstableElicitationRequestScope = ElicitationRequestScope

// Deprecated: Use ElicitationSchema.
type UnstableElicitationSchema = ElicitationSchema

// Deprecated: Use ElicitationSchemaType.
type UnstableElicitationSchemaType = ElicitationSchemaType

// Deprecated: Use ElicitationSessionScope.
type UnstableElicitationSessionScope = ElicitationSessionScope

// Deprecated: Use ElicitationUrlModeSession.
type UnstableElicitationUrlModeSession = ElicitationUrlModeSession

// Deprecated: Use ElicitationUrlModeRequest.
type UnstableElicitationUrlModeRequest = ElicitationUrlModeRequest

// Deprecated: Use ElicitationUrlMode.
type UnstableElicitationUrlMode = ElicitationUrlMode

// Deprecated: Use ElicitationSchemaTypeObject.
const UnstableElicitationSchemaTypeObject = ElicitationSchemaTypeObject

// Deprecated: Use NewCreateElicitationRequestFormSession.
func NewUnstableCreateElicitationRequestFormSession(message string, requestedSchema UnstableElicitationSchema, sessionId SessionId) UnstableCreateElicitationRequest {
	return UnstableCreateElicitationRequest{FormSession: &UnstableCreateElicitationFormSession{
		Message:         message,
		Mode:            "form",
		RequestedSchema: requestedSchema,
		SessionId:       sessionId,
	}}
}

// Deprecated: Use NewCreateElicitationRequestFormRequest.
func NewUnstableCreateElicitationRequestFormRequest(message string, requestId RequestId, requestedSchema UnstableElicitationSchema) UnstableCreateElicitationRequest {
	return UnstableCreateElicitationRequest{FormRequest: &UnstableCreateElicitationFormRequest{
		Message:         message,
		Mode:            "form",
		RequestId:       requestId,
		RequestedSchema: requestedSchema,
	}}
}

// Deprecated: Use NewCreateElicitationRequestUrlSession.
func NewUnstableCreateElicitationRequestUrlSession(elicitationId UnstableElicitationId, message string, sessionId SessionId, url string) UnstableCreateElicitationRequest {
	return UnstableCreateElicitationRequest{UrlSession: &UnstableCreateElicitationUrlSession{
		ElicitationId: elicitationId,
		Message:       message,
		Mode:          "url",
		SessionId:     sessionId,
		Url:           url,
	}}
}

// Deprecated: Use NewCreateElicitationRequestUrlRequest.
func NewUnstableCreateElicitationRequestUrlRequest(elicitationId UnstableElicitationId, message string, requestId RequestId, url string) UnstableCreateElicitationRequest {
	return UnstableCreateElicitationRequest{UrlRequest: &UnstableCreateElicitationUrlRequest{
		ElicitationId: elicitationId,
		Message:       message,
		Mode:          "url",
		RequestId:     requestId,
		Url:           url,
	}}
}

// Deprecated: Use NewCreateElicitationRequestOtherSession.
func NewUnstableCreateElicitationRequestOtherSession(message string, mode string, sessionId SessionId) UnstableCreateElicitationRequest {
	return UnstableCreateElicitationRequest{OtherSession: &UnstableCreateElicitationOtherSession{
		Message:   message,
		Mode:      mode,
		SessionId: sessionId,
	}}
}

// Deprecated: Use NewCreateElicitationRequestOtherRequest.
func NewUnstableCreateElicitationRequestOtherRequest(message string, mode string, requestId RequestId) UnstableCreateElicitationRequest {
	return UnstableCreateElicitationRequest{OtherRequest: &UnstableCreateElicitationOtherRequest{
		Message:   message,
		Mode:      mode,
		RequestId: requestId,
	}}
}

// Deprecated: Use NewCreateElicitationResponseAccept.
func NewUnstableCreateElicitationResponseAccept() UnstableCreateElicitationResponse {
	return UnstableCreateElicitationResponse{Accept: &UnstableCreateElicitationAccept{Action: "accept"}}
}

// Deprecated: Use NewCreateElicitationResponseDecline.
func NewUnstableCreateElicitationResponseDecline() UnstableCreateElicitationResponse {
	return UnstableCreateElicitationResponse{Decline: &UnstableCreateElicitationDecline{Action: "decline"}}
}

// Deprecated: Use NewCreateElicitationResponseCancel.
func NewUnstableCreateElicitationResponseCancel() UnstableCreateElicitationResponse {
	return UnstableCreateElicitationResponse{Cancel: &UnstableCreateElicitationCancel{Action: "cancel"}}
}

// Deprecated: Use NewCreateElicitationResponseOther.
func NewUnstableCreateElicitationResponseOther(action string) UnstableCreateElicitationResponse {
	return UnstableCreateElicitationResponse{Other: &UnstableCreateElicitationOther{Action: action}}
}

// Deprecated: Use NewElicitationFormModeSession.
func NewUnstableElicitationFormModeSession(requestedSchema UnstableElicitationSchema, sessionId SessionId) UnstableElicitationFormMode {
	return UnstableElicitationFormMode{Session: &UnstableElicitationFormModeSession{
		RequestedSchema: requestedSchema,
		SessionId:       sessionId,
	}}
}

// Deprecated: Use NewElicitationFormModeRequest.
func NewUnstableElicitationFormModeRequest(requestId RequestId, requestedSchema UnstableElicitationSchema) UnstableElicitationFormMode {
	return UnstableElicitationFormMode{Request: &UnstableElicitationFormModeRequest{
		RequestId:       requestId,
		RequestedSchema: requestedSchema,
	}}
}

// Deprecated: Use NewElicitationUrlModeSession.
func NewUnstableElicitationUrlModeSession(elicitationId UnstableElicitationId, sessionId SessionId, url string) UnstableElicitationUrlMode {
	return UnstableElicitationUrlMode{Session: &UnstableElicitationUrlModeSession{
		ElicitationId: elicitationId,
		SessionId:     sessionId,
		Url:           url,
	}}
}

// Deprecated: Use NewElicitationUrlModeRequest.
func NewUnstableElicitationUrlModeRequest(elicitationId UnstableElicitationId, requestId RequestId, url string) UnstableElicitationUrlMode {
	return UnstableElicitationUrlMode{Request: &UnstableElicitationUrlModeRequest{
		ElicitationId: elicitationId,
		RequestId:     requestId,
		Url:           url,
	}}
}

// Deprecated: Use CreateElicitation.
func (c *AgentSideConnection) UnstableCreateElicitation(ctx context.Context, params UnstableCreateElicitationRequest) (UnstableCreateElicitationResponse, error) {
	return c.CreateElicitation(ctx, params)
}

// Deprecated: Use CompleteElicitation.
func (c *AgentSideConnection) UnstableCompleteElicitation(ctx context.Context, params UnstableCompleteElicitationNotification) error {
	return c.CompleteElicitation(ctx, params)
}
