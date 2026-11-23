package v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coder/acp-go-sdk"
)

// LegacyPeerError identifies an initialize reply from an ACP v1 peer.
// The application can open a fresh connection with the v1 client.
type LegacyPeerError struct {
	cause *acp.RequestError
}

func (e *LegacyPeerError) Error() string {
	if e.cause != nil {
		return "peer supports ACP v1: " + e.cause.Error()
	}
	return "peer selected ACP v1"
}

func (e *LegacyPeerError) Unwrap() error {
	if e.cause == nil {
		return nil
	}
	return e.cause
}

func decodeInitializeResponse(raw json.RawMessage) (InitializeResponse, error) {
	var response InitializeResponse
	obj, err := readObject(raw)
	if err != nil {
		return response, fmt.Errorf("initialize result: %w", err)
	}
	if err := obj.require("protocolVersion"); err != nil {
		return response, fmt.Errorf("initialize result: %w", err)
	}
	var version uint16
	if err := json.Unmarshal(obj["protocolVersion"], &version); err != nil {
		return response, fmt.Errorf("initialize protocolVersion: %w", err)
	}
	_, hasInfo := obj["info"]
	_, hasCapabilities := obj["capabilities"]
	_, hasLegacyInfo := obj["agentInfo"]
	_, hasLegacyCapabilities := obj["agentCapabilities"]
	hasV2Fields := hasInfo || hasCapabilities
	hasLegacyFields := hasLegacyInfo || hasLegacyCapabilities

	if hasV2Fields && (hasLegacyFields || version == acp.ProtocolVersionNumber) {
		return response, errors.New("initialize result mixes ACP v1 and v2 fields")
	}
	switch version {
	case acp.ProtocolVersionNumber, ProtocolVersionNumber:
	default:
		return response, fmt.Errorf("unsupported initialize protocolVersion %d", version)
	}

	if metadata := obj["_meta"]; metadata != nil && !bytes.Equal(metadata, []byte("null")) {
		if _, err := readObject(metadata); err != nil {
			return response, fmt.Errorf("initialize _meta: %w", err)
		}
	}
	if version == acp.ProtocolVersionNumber || hasLegacyFields {
		recognized, err := validateLegacyResult(obj)
		if err != nil {
			return response, fmt.Errorf("legacy initialize result: %w", err)
		}
		if recognized || version == acp.ProtocolVersionNumber {
			return response, &LegacyPeerError{}
		}
	}

	if err := obj.require("info"); err != nil {
		return response, fmt.Errorf("initialize result: %w", err)
	}
	for _, field := range []string{"capabilities", "authMethods"} {
		if _, present := obj[field]; present {
			if err := obj.require(field); err != nil {
				return response, fmt.Errorf("initialize result: %w", err)
			}
		}
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return InitializeResponse{}, fmt.Errorf("initialize result: %w", err)
	}
	response.ProtocolVersion = version
	return response, nil
}

func validateLegacyResult(obj wireObject) (bool, error) {
	recognized := false
	if raw := obj["agentInfo"]; raw != nil && !bytes.Equal(raw, []byte("null")) {
		if _, err := decodeObject[Implementation](raw); err != nil {
			return false, fmt.Errorf("agentInfo: %w", err)
		}
		recognized = true
	}
	if raw := obj["agentCapabilities"]; raw != nil {
		// Bypass the v1 decoder's defaults so a malformed object cannot
		// supply evidence of a legacy envelope.
		type capabilities acp.AgentCapabilities
		if _, err := decodeObject[capabilities](raw); err != nil {
			return false, fmt.Errorf("agentCapabilities: %w", err)
		}
		recognized = true
	}
	if raw := obj["authMethods"]; raw != nil {
		if err := obj.require("authMethods"); err != nil {
			return false, err
		}
		var methods []json.RawMessage
		if err := json.Unmarshal(raw, &methods); err != nil {
			return false, fmt.Errorf("authMethods: %w", err)
		}
		for _, method := range methods {
			if _, err := decodeObject[struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}](method, "id", "name"); err != nil {
				return false, fmt.Errorf("authMethods: %w", err)
			}
		}
	}
	return recognized, nil
}

func classifyInitializeError(err error) error {
	var peer *acp.RequestError
	if !errors.As(err, &peer) || !peer.IsRemoteResponse() || peer.Code != -32022 {
		return err
	}
	data, ok := peer.Data.(map[string]any)
	if !ok {
		return err
	}
	requested, ok := parseProtocolVersion(data["requested"])
	if !ok || requested != ProtocolVersionNumber {
		return err
	}
	supported, ok := data["supported"].([]any)
	if !ok {
		return err
	}
	legacy := false
	for _, raw := range supported {
		version, ok := parseProtocolVersion(raw)
		if !ok || version == requested {
			return err
		}
		legacy = legacy || version == acp.ProtocolVersionNumber
	}
	if legacy {
		return &LegacyPeerError{cause: peer}
	}
	return err
}

func parseProtocolVersion(value any) (uint16, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	version, err := number.Int64()
	if err != nil || version < 0 || version > 65535 {
		return 0, false
	}
	return uint16(version), true
}
