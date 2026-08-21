package acp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCreateElicitationRequestScopeRoundTrip(t *testing.T) {
	schema := UnstableElicitationSchema{Properties: map[string]any{}, Type: UnstableElicitationSchemaTypeObject}
	requestID := requestIDString("request-1")
	toolCallID := ToolCallId("tool-1")

	formSession := NewUnstableCreateElicitationRequestFormSession("Choose values", schema, "session-1")
	formSession.FormSession.ToolCallId = &toolCallID
	tests := []struct {
		name          string
		request       UnstableCreateElicitationRequest
		expectedScope string
	}{
		{name: "form session", request: formSession, expectedScope: "sessionId"},
		{name: "form request", request: NewUnstableCreateElicitationRequestFormRequest("Choose values", requestID, schema), expectedScope: "requestId"},
		{name: "url session", request: NewUnstableCreateElicitationRequestUrlSession("elicitation-1", "Open the URL", "session-1", "https://example.com/input"), expectedScope: "sessionId"},
		{name: "url request", request: NewUnstableCreateElicitationRequestUrlRequest("elicitation-1", "Open the URL", requestID, "https://example.com/input"), expectedScope: "requestId"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.request.Validate(); err != nil {
				t.Fatalf("Validate before marshal: %v", err)
			}
			encoded, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatalf("decode wire object: %v", err)
			}
			if _, ok := wire[tt.expectedScope]; !ok {
				t.Fatalf("wire payload %s does not contain %q", encoded, tt.expectedScope)
			}
			oppositeScope := "requestId"
			if tt.expectedScope == "requestId" {
				oppositeScope = "sessionId"
			}
			if _, ok := wire[oppositeScope]; ok {
				t.Fatalf("wire payload %s unexpectedly contains %q", encoded, oppositeScope)
			}

			var decoded UnstableCreateElicitationRequest
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatalf("Validate after unmarshal: %v", err)
			}
			reencoded, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("Marshal decoded request: %v", err)
			}
			var reencodedWire map[string]any
			var originalWire map[string]any
			if err := json.Unmarshal(encoded, &originalWire); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(reencoded, &reencodedWire); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reencodedWire, originalWire) {
				t.Fatalf("round-trip mismatch: original=%s reencoded=%s", encoded, reencoded)
			}
		})
	}
}

func TestCreateElicitationRequestRejectsInvalidScope(t *testing.T) {
	schema := UnstableElicitationSchema{Properties: map[string]any{}, Type: UnstableElicitationSchemaTypeObject}
	missingSession := NewUnstableCreateElicitationRequestFormSession("Choose values", schema, "")
	if err := missingSession.Validate(); err == nil {
		t.Fatal("session-scoped request with an empty sessionId passed validation")
	}
	missingRequest := NewUnstableCreateElicitationRequestFormRequest("Choose values", RequestId{}, schema)
	if err := missingRequest.Validate(); err == nil {
		t.Fatal("request-scoped request with an empty requestId passed validation")
	}

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "form missing scope",
			payload: `{"mode":"form","message":"Choose values","requestedSchema":{}}`,
		},
		{
			name:    "form ambiguous scope",
			payload: `{"mode":"form","message":"Choose values","requestedSchema":{},"sessionId":"session-1","requestId":"request-1"}`,
		},
		{
			name:    "url missing scope",
			payload: `{"mode":"url","message":"Open URL","elicitationId":"elicitation-1","url":"https://example.com/input"}`,
		},
		{
			name:    "url ambiguous scope",
			payload: `{"mode":"url","message":"Open URL","elicitationId":"elicitation-1","url":"https://example.com/input","sessionId":"session-1","requestId":"request-1"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request UnstableCreateElicitationRequest
			if err := json.Unmarshal([]byte(tt.payload), &request); err != nil {
				t.Fatalf("Unmarshal should preserve validation failure: %v", err)
			}
			if err := request.Validate(); err == nil {
				t.Fatal("invalid scope passed validation")
			}
		})
	}
}

func TestElicitationAnyOfHelpersMergeParentFields(t *testing.T) {
	schema := UnstableElicitationSchema{Properties: map[string]any{}, Type: UnstableElicitationSchemaTypeObject}
	requestID := requestIDString("request-1")
	tests := []struct {
		name     string
		validate func() error
	}{
		{name: "form session", validate: func() error {
			value := NewUnstableElicitationFormModeSession(schema, "session-1")
			return value.Validate()
		}},
		{name: "form request", validate: func() error {
			value := NewUnstableElicitationFormModeRequest(requestID, schema)
			return value.Validate()
		}},
		{name: "url session", validate: func() error {
			value := NewUnstableElicitationUrlModeSession("elicitation-1", "session-1", "https://example.com/input")
			return value.Validate()
		}},
		{name: "url request", validate: func() error {
			value := NewUnstableElicitationUrlModeRequest("elicitation-1", requestID, "https://example.com/input")
			return value.Validate()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.validate(); err != nil {
				t.Fatalf("generated helper returned invalid union: %v", err)
			}
		})
	}
}

func TestCreateElicitationCustomModesPreserveExtensionFields(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectedScope string
		extra         func(UnstableCreateElicitationRequest) map[string]json.RawMessage
	}{
		{
			name:          "session scope",
			expectedScope: "sessionId",
			payload:       `{"mode":"_vendor/custom","message":"Custom input","sessionId":"session-1","toolCallId":"tool-1","flag":true,"count":3,"object":{"nested":"value"},"array":[1,"two",false],"nothing":null}`,
			extra: func(request UnstableCreateElicitationRequest) map[string]json.RawMessage {
				if request.OtherSession == nil {
					t.Fatal("custom session payload did not select OtherSession")
				}
				return request.OtherSession.Extra
			},
		},
		{
			name:          "request scope",
			expectedScope: "requestId",
			payload:       `{"mode":"future_mode","message":"Custom input","requestId":"request-1","enabled":false,"ratio":1.25,"object":{"nested":{"depth":2}},"array":[{"id":1},null],"nothing":null}`,
			extra: func(request UnstableCreateElicitationRequest) map[string]json.RawMessage {
				if request.OtherRequest == nil {
					t.Fatal("custom request payload did not select OtherRequest")
				}
				return request.OtherRequest.Extra
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request UnstableCreateElicitationRequest
			if err := json.Unmarshal([]byte(tt.payload), &request); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if err := request.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			extra := tt.extra(request)
			for _, key := range []string{"object", "array", "nothing"} {
				if _, ok := extra[key]; !ok {
					t.Errorf("Extra does not contain %q: %#v", key, extra)
				}
			}
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var original map[string]any
			var roundTripped map[string]any
			if err := json.Unmarshal([]byte(tt.payload), &original); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &roundTripped); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, roundTripped) {
				t.Fatalf("custom payload changed: original=%s round-tripped=%s", tt.payload, encoded)
			}
			if _, ok := roundTripped[tt.expectedScope]; !ok {
				t.Fatalf("round-tripped payload lost %q", tt.expectedScope)
			}
		})
	}
}

func TestCreateElicitationOtherModeValidation(t *testing.T) {
	requestID := requestIDString("request-1")
	tests := []struct {
		name    string
		request UnstableCreateElicitationRequest
		valid   bool
	}{
		{name: "future session mode", request: NewUnstableCreateElicitationRequestOtherSession("Custom", "future_mode", "session-1"), valid: true},
		{name: "extension request mode", request: NewUnstableCreateElicitationRequestOtherRequest("Custom", "_vendor/custom", requestID), valid: true},
		{name: "empty mode", request: NewUnstableCreateElicitationRequestOtherSession("Custom", "", "session-1")},
		{name: "reserved form", request: NewUnstableCreateElicitationRequestOtherSession("Custom", "form", "session-1")},
		{name: "reserved url", request: NewUnstableCreateElicitationRequestOtherRequest("Custom", "url", requestID)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.valid && err != nil {
				t.Fatalf("future mode failed validation: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("invalid custom mode passed validation")
			}
		})
	}

	decodedTests := []struct {
		name   string
		data   string
		target interface{ Validate() error }
	}{
		{name: "decoded form", data: `{"mode":"form","message":"Custom","sessionId":"session-1"}`, target: &UnstableCreateElicitationOtherSession{}},
		{name: "decoded url", data: `{"mode":"url","message":"Custom","requestId":"request-1"}`, target: &UnstableCreateElicitationOtherRequest{}},
	}
	for _, tt := range decodedTests {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.data), tt.target); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if err := tt.target.Validate(); err == nil {
				t.Fatal("decoded reserved custom mode passed validation")
			}
		})
	}
}

func requestIDString(value string) RequestId {
	id := RequestIdStr(value)
	return RequestId{Str: &id}
}
