# ACP v2 preview client design

```yaml
status: draft
requirements:
  - REQ-SDK-ACP-V2-001
  - REQ-SDK-ACP-V2-002
  - REQ-SDK-ACP-V2-003
  - REQ-SDK-ACP-V2-004
```

## Package and schema

Use `github.com/coder/acp-go-sdk/experimental/v2` in the existing module.
The root package owns the stable v1 API and shared JSON-RPC transport.
Reuse `acp.NewConnection`, `acp.SendRequest[T]`, `SendNotification`,
`ConnectionOption`, and `RequestError`. The caller owns the pipes and
the subprocess.

Keep the Preview schema under `schema/v2/`. Pin the stable v2 schema and
method metadata to `schema-v2.0.0-beta.1`, commit
`c0945895a63233bfdbdf95990104c11f5d8d8991`. Record the source and hashes in
that directory. The `schema/version` and root `version` stamps track v1.
[Upstream release](https://github.com/agentclientprotocol/agent-client-protocol/releases/tag/schema-v2.0.0-beta.1)

| Pinned artifact | SHA-256 |
| --- | --- |
| [`schema/v2/schema.json`](https://raw.githubusercontent.com/agentclientprotocol/agent-client-protocol/c0945895a63233bfdbdf95990104c11f5d8d8991/schema/v2/schema.json) | `4e5c50f5c3e40363437d4804f932ee2939cff25aa16e2882991430aa8102d5ad` |
| [`schema/v2/meta.json`](https://raw.githubusercontent.com/agentclientprotocol/agent-client-protocol/c0945895a63233bfdbdf95990104c11f5d8d8991/schema/v2/meta.json) | `5eac58a28615d399af1db8e3d636767a7e404f2df38c9668a1ba00b1989e0714` |

Implement the consumed types by hand for the pilot. The existing generator
emits duplicate v2 state fields and v1 `LegacyModels` references when run
against the full Preview schema. A generator upgrade is separate work.

## Client surface

Follow the root API's naming and construction patterns. Use a
`ClientSideConnection` with a typed `Initialize` and the methods below.
Return the decoded RPC response and preserve correlated peer errors.

| SDK call | Wire method and contract |
| --- | --- |
| `Initialize` | `initialize`. Request version 2 with required `info`. Return v2 `info` and optional capabilities and auth methods. |
| `LoginAuth`, `LogoutAuth` | `auth/login`, `auth/logout`. Use `methodId` for auth methods and login. |
| `NewSession`, `ResumeSession` | `session/new`, `session/resume`. Carry `cwd`, MCP servers, and additional directories. Resume takes `sessionId` and optional `replayFrom`. |
| `ListSessions`, `CloseSession` | `session/list`, `session/close`. List keeps cursor pagination. Close returns the RPC result. |
| `SetSessionConfigOption` | `session/set_config_option`. Use `configId` and the `select` or `boolean` value discriminator. |
| `Prompt` | `session/prompt`. Send content and an optional `idempotencyKey`. Require a `messageId` result. |
| `Cancel` | `session/cancel`. Send a notification with `sessionId`. |

Use typed HTTP and stdio MCP server values. Each carries its own `type`.
The caller supplies unique names, resolves absolute command paths, and selects
advertised transports. Send the full server and root lists again on resume.
[Session setup](https://agentclientprotocol.com/protocol/v2/session-setup)

Expose `session/update` through a typed handler. Optional handlers cover
`session/request_permission`, `elicitation/create`, and `elicitation/complete`.
Check the direction before invoking a handler. The root transport already
stores the inbound request bit in its handler context. Expose it through a
small exported accessor such as `acp.IsInboundRequest(ctx)`.

## Initialization and error classification

Decode the initialize result as raw JSON before choosing its shape. A
successful v2 result requires version 2 and a valid `info` object with
`name` and `version`. Optional capabilities may be absent. Latch the result
for this connection. Guard concurrent initialization and gate session calls
without holding a lock across I/O or callbacks.

Return a typed `LegacyPeerError` in these cases:

1. Version 1 is selected in a valid v1 result, including a minimal v1 result.
1. Version 2 is stamped on a result with a recognizable v1 envelope.
   The v2 `info` and `capabilities` keys are absent. At least one of
   `agentInfo` and `agentCapabilities` is a valid legacy object.
1. A correlated `-32022` reply to this initialize request lists v1 as
   supported. Require structured version data consistent with the request.
   Preserve the peer error through `Unwrap`.

Return an ordinary error for ambiguous envelopes, malformed v2 fields,
unknown versions, other peer errors, and transport failures. The application
can use `errors.As` to identify `LegacyPeerError` and open a fresh v1 pipe.
[Initialization](https://agentclientprotocol.com/protocol/v2/initialization)

## Update decoding and response order

Decode `sessionUpdate` as a tagged union. Preserve the type and raw payload
for future variants. Model the fields that Kandev consumes from the current
standard updates. Both IDs must survive when a parent update references
a child session.

Use the fork's `acp.Optional[T]` or equivalent field presence tracking
for patches. For fields the SDK marshals, explicitly omit unset values on
Go 1.21. Test omitted, null, empty, and populated values. Preserve opaque
`_meta` values and open enum strings.
[Extensibility](https://agentclientprotocol.com/protocol/v2/extensibility)

The reader queues notifications in wire order. The transport waits for
earlier notification handlers before returning a response to the caller.
The v2 client inherits this barrier. A later update can arrive after the
corresponding call returns. In particular, `Prompt` returns the message ID,
and `state_update` can report the outcome later. `ResumeSession` exposes
`replay_complete` on the update stream. The application owns turn
correlation, replay reconciliation, and retry decisions.
[Prompt lifecycle](https://agentclientprotocol.com/protocol/v2/prompt-lifecycle)

## Callback safety

Dispatch permission and elicitation calls with the transport's request
context. Validate request shape before invoking the handler. Return
`-32601` for unsupported requests and `-32602` for malformed supported
requests. Preserve a handler's explicit protocol error. An inbound
`$/cancel_request` cancels the matching request context.

## Fixture examples

These frames use synthetic names and IDs. The initialize results show the
two response shapes that matter for the v2 handshake.

```jsonl
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":2,"info":{"name":"fixture","version":"1.0"},"capabilities":{"session":{"mcp":{"stdio":{},"http":{}}}}}}
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":2,"agentInfo":{"name":"legacy","version":"1.0"},"agentCapabilities":{}}}
```

Use the first frame for success and the second for `LegacyPeerError`.
For the prompt test, send the acknowledgement while the agent still has work.

```jsonl
{"jsonrpc":"2.0","id":3,"result":{"messageId":"user-1"}}
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"session-1","update":{"sessionUpdate":"agent_message_chunk","messageId":"agent-1","content":{"type":"text","text":"fixture answer"}}}}
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"session-1","update":{"sessionUpdate":"state_update","state":"idle","stopReason":"end_turn"}}}
```

## Verification mapping

| Requirement | Verification |
| --- | --- |
| `REQ-SDK-ACP-V2-001` | Negotiation cases and session call gate over paired pipes |
| `REQ-SDK-ACP-V2-002` | Payload goldens and prompt, resume, and cancel transport tests |
| `REQ-SDK-ACP-V2-003` | Patch, future variant, callback, cancellation, and notification tests |
| `REQ-SDK-ACP-V2-004` | Stored schema hashes, Go 1.21 check, and the root SDK suite |
