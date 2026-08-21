package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestLegacyModelsDecodeFromSessionResponses(t *testing.T) {
	const payload = `{
		"sessionId":"session-1",
		"models":{
			"availableModels":[{"modelId":"legacy-model","name":"Legacy Model"}],
			"currentModelId":"legacy-model"
		}
	}`
	var newSessionResponse NewSessionResponse
	var loadSessionResponse LoadSessionResponse
	var forkSessionResponse UnstableForkSessionResponse

	tests := []struct {
		name   string
		target any
		models func() *LegacyModels
	}{
		{
			name:   "new session",
			target: &newSessionResponse,
			models: func() *LegacyModels {
				return newSessionResponse.LegacyModels
			},
		},
		{
			name:   "load session",
			target: &loadSessionResponse,
			models: func() *LegacyModels {
				return loadSessionResponse.LegacyModels
			},
		},
		{
			name:   "fork session",
			target: &forkSessionResponse,
			models: func() *LegacyModels {
				return forkSessionResponse.LegacyModels
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(payload), tt.target); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			models := tt.models()
			if models == nil {
				t.Fatal("LegacyModels is nil")
			}
			if models.CurrentModelId != "legacy-model" {
				t.Fatalf("CurrentModelId = %q, want legacy-model", models.CurrentModelId)
			}
			if len(models.AvailableModels) != 1 || models.AvailableModels[0].ModelId != "legacy-model" {
				t.Fatalf("AvailableModels = %#v", models.AvailableModels)
			}
		})
	}
}

func TestUnstableSetSessionModelUsesLegacyWireMethod(t *testing.T) {
	requestReader, requestWriter := io.Pipe()
	responseReader, responseWriter := io.Pipe()
	t.Cleanup(func() {
		_ = requestReader.Close()
		_ = requestWriter.Close()
		_ = responseReader.Close()
		_ = responseWriter.Close()
	})

	conn := NewClientSideConnection(&clientFuncs{}, requestWriter, responseReader)
	serverErr := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(requestReader).ReadBytes('\n')
		if err != nil {
			serverErr <- err
			return
		}
		var request struct {
			ID     json.RawMessage                `json:"id"`
			Method string                         `json:"method"`
			Params UnstableSetSessionModelRequest `json:"params"`
		}
		if err := json.Unmarshal(line, &request); err != nil {
			serverErr <- err
			return
		}
		if request.Method != LegacyAgentMethodSessionSetModel {
			serverErr <- &legacyWireMethodError{got: request.Method}
			return
		}
		if request.Params.SessionId != "session-1" || request.Params.ModelId != "legacy-model" {
			serverErr <- &legacyWireParamsError{params: request.Params}
			return
		}
		response, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  map[string]any{},
		})
		if err == nil {
			response = append(response, '\n')
			_, err = responseWriter.Write(response)
		}
		serverErr <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := conn.UnstableSetSessionModel(ctx, UnstableSetSessionModelRequest{
		SessionId: "session-1",
		ModelId:   "legacy-model",
	}); err != nil {
		t.Fatalf("UnstableSetSessionModel: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

type legacyWireMethodError struct{ got string }

func (e *legacyWireMethodError) Error() string {
	return "legacy model request used unexpected method: " + e.got
}

type legacyWireParamsError struct {
	params UnstableSetSessionModelRequest
}

func (e *legacyWireParamsError) Error() string {
	data, _ := json.Marshal(e.params)
	return "legacy model request used unexpected params: " + string(data)
}
