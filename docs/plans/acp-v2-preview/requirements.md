# ACP v2 preview client requirements

```yaml
status: draft
created: 2026-11-23
```

This package gives Go applications a typed client for the ACP v2 Preview
wire protocol. Kandev is the first consumer. The preview client and the
stable ACP v1 API share the same Go module.

## Requirements

### REQ-SDK-ACP-V2-001: Select a valid protocol

- **AC-SDK-ACP-V2-001.1:** The client sends version 2 with v2 client identity
  and capabilities. A valid v2 response enables v2 session calls.
- **AC-SDK-ACP-V2-001.2:** A caller can distinguish a recognized v1 selection
  from a malformed response, provider error, deadline, or disconnect.
- **AC-SDK-ACP-V2-001.3:** Session calls on an uninitialized connection
  fail before they reach the agent.

### REQ-SDK-ACP-V2-002: Send typed session calls

- **AC-SDK-ACP-V2-002.1:** A caller can send v2 auth, setup, configuration,
  prompt, resume, close, and cancel messages through the client API.
- **AC-SDK-ACP-V2-002.2:** The prompt response returns the agent's message
  ID at acceptance. The caller receives foreground outcomes and replay
  boundaries as separate notifications.
- **AC-SDK-ACP-V2-002.3:** Session setup carries the supplied MCP servers and
  workspace roots in the v2 wire format.

### REQ-SDK-ACP-V2-003: Preserve inbound v2 information

- **AC-SDK-ACP-V2-003.1:** The caller receives session updates in wire
  order, including updates sent before their related response.
- **AC-SDK-ACP-V2-003.2:** Update decoding distinguishes omitted fields,
  explicit nulls, and concrete values. Well-formed future variants reach
  the caller without losing their payload.
- **AC-SDK-ACP-V2-003.3:** Permission and elicitation requests reach only
  their registered handlers. Cancellation reaches the handler context.

### REQ-SDK-ACP-V2-004: Keep Preview opt-in

- **AC-SDK-ACP-V2-004.1:** Applications can import the Preview package on
  Go 1.21. The existing v1 test suite and examples still pass.
- **AC-SDK-ACP-V2-004.2:** The package records the upstream schema version
  and runs its wire and transport tests offline.

## Scope boundary

This package covers the typed client and its transport binding. Provider
selection, process restart, credentials, user interaction policy, turn
settlement, and transcript projection belong to the application.

## Related documents

[System design](system-design.md) · [Work package](plan.md)
