# Add the v2 client connection and interoperability tests

```yaml
id: 02-client-connection
status: complete
wave: 2
depends_on:
  - 01-schema-and-wire-types
plan: plan.md
requirements:
  - REQ-SDK-ACP-V2-001
  - REQ-SDK-ACP-V2-002
  - REQ-SDK-ACP-V2-003
  - REQ-SDK-ACP-V2-004
acceptance_criteria:
  - AC-SDK-ACP-V2-001.1
  - AC-SDK-ACP-V2-001.2
  - AC-SDK-ACP-V2-001.3
  - AC-SDK-ACP-V2-002.1
  - AC-SDK-ACP-V2-002.2
  - AC-SDK-ACP-V2-002.3
  - AC-SDK-ACP-V2-003.1
  - AC-SDK-ACP-V2-003.3
  - AC-SDK-ACP-V2-004.1
  - AC-SDK-ACP-V2-004.2
system_design: system-design.md
```

Bind the v2 payloads to the existing JSON-RPC transport. Exercise the
public client API against a scripted peer over paired pipes.

## Scope

- Add `NewClientSideConnection` and the typed methods in the
  [system design](system-design.md). Expose connection termination through
  `Done` and pass the existing transport options through.
- Validate initialization and return `LegacyPeerError` for the cases in the
  design. Keep the version selection on the client connection. Reject session
  traffic before initialization succeeds.
- Dispatch callbacks by method and direction. Keep request contexts and
  response provenance from the shared transport.
- Document the experimental import path, transport ownership, and how to
  handle v1 selection in `README.md` and `FORK.md`.

## Acceptance

1. The loopback tests distinguish v2 success, v1 selection, a v1 envelope
   stamped 2, a compatible version error, and other failures. Failed
   negotiation sends no session calls.
1. Prompt acknowledgement returns before the held idle update. Earlier
   updates and replay markers reach handlers in order. Session cancel sends
   a notification and later foreground state remains observable.
1. Callback tests prove request direction, request cancellation, and
   correlated replies. The root v1 tests and examples pass.

## Verification

From the SDK repository root:

```bash
go test -trimpath ./experimental/v2 -run '^TestClient_' -count=1
go test -trimpath -race ./experimental/v2/... -count=1
GOTOOLCHAIN=go1.21.13 go test -trimpath ./...
make test
git diff --check
```

These tests cover the client contract.

| Test | Evidence |
| --- | --- |
| `TestClient_Initialize` | Valid v2 with optional fields, minimal v1, v1-stamped-as-v2, unknown version, malformed `info`, conflicting fields, version error with and without v1, and disconnect. |
| `TestClient_SessionCalls` | Session RPC method names and wire payloads. Calls before successful initialization do not reach the peer. |
| `TestClient_PromptAcknowledgement` | Acknowledgement and `running` in both orders. Hold the terminal update until the public call returns. |
| `TestClient_ReplayOrdering` | Replay updates and `replay_complete` before a successful resume response are visible to the caller. |
| `TestClient_CancelSession` | An outbound `session/cancel` frame carries no request ID. A later idle cancellation update still reaches the handler. |
| `TestClient_Callbacks` | Permission and elicitation request IDs remain correlated. Wrong direction and invalid payloads do not invoke handlers. A peer cancellation reaches the matching context. |
| `TestClient_ErrorProvenance` | Only a correlated peer error can classify as version fallback. A malformed response or local failure remains distinguishable. |

Use channels and context deadlines to coordinate pipe tests. Close both
pipe directions in test cleanup. Use the root connection barrier and
cancellation tests as examples.

## Files and dependencies

Likely paths include `experimental/v2/client.go`, `experimental/v2/dispatch.go`,
and the corresponding tests. `connection.go` can expose the existing request
direction bit through a small, additive accessor.

The first work order supplies the types. Report the tested revision and
public APIs to the Kandev integrator.

## Results

The tested worktree is
`/root/.kandev/tasks/implement-the-acp-v2_2mfcnwir/kdlbs-acp-go-sdk`.
Its base revision is `ec6d622b199996ceb8f44544129f7895d88e7f38`.
Use the worktree as a local replacement for `github.com/coder/acp-go-sdk`.
The import path is `github.com/coder/acp-go-sdk/experimental/v2`.

`v2.NewClientSideConnection` accepts `v2.ClientHandlers`, the agent input
and output streams, and optional `acp.ConnectionOption` values.
The handlers are `SessionUpdate`, `RequestPermission`, `CreateElicitation`,
and `CompleteElicitation`. `Done` signals that the transport stopped reading.
`NotificationsDone` closes after queued notification handlers return.

- `Initialize` negotiates v2. Use `errors.As` with `*v2.LegacyPeerError` to
  detect v1 selection. Open a fresh connection for the v1 client.
- `LoginAuth`, `LogoutAuth`, `NewSession`, `ResumeSession`, `ListSessions`,
  `CloseSession`, `SetSessionConfigOption`, `Prompt`, and `Cancel` use typed
  v2 payloads. Calls before successful initialization return
  `v2.ErrNotInitialized`.
- `Prompt` returns `PromptResponse.MessageID` at acceptance.
  Foreground state and replay boundaries arrive through `SessionUpdate`.
  Use `Update.Type()` for the discriminator and `Update.Value` for the payload.

### Verification

- `go test -trimpath ./experimental/v2 -run '^TestClient_' -count=1` passed.
- The EOF and log redaction tests passed 20 runs. The EOF test delivered
  the final idle update before `NotificationsDone` closed.
- `go test -trimpath -race ./experimental/v2/... -count=1` passed.
- `GOTOOLCHAIN=go1.21.13 go test -trimpath ./...` passed.
- `mise exec -- make test` passed with Go 1.26.3. The command ran the v1
  suite and generator tests, and built all examples.
- `mise exec -- make fmt` and `git diff --check` passed.
