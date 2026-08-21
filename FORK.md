# Fork changes vs upstream

This repository is a fork of [`coder/acp-go-sdk`](https://github.com/coder/acp-go-sdk),
maintained for [Kandev](https://github.com/kdlbs/kandev). Its bindings are
generated from ACP schema 1.20.0, and it adds the changes below. Everything here
exists because Kandev needs behavior that upstream does not provide; keep this
file current when the fork's delta changes.

## Why the fork exists

Kandev talks to several ACP agents (Claude Code, Gemini, Cursor, and others) that
do not all track the latest ACP schema. Two of them, plus one Cursor-specific
protocol quirk, forced changes the upstream SDK rejects by design:

1. Cursor emits a non-standard `cursor/task` client request that upstream refuses
   because it is not underscore-prefixed.
1. Older agents still send pre-`v0.13.5` payloads (top-level `models`, the legacy
   `session/set_model` method) that upstream dropped.
1. Kandev needs a bounded inbound notification queue with a fail-fast overflow
   signal, which upstream does not expose.

## Changes

### 1. Vendor client-request extension seam (the primary reason for the fork)

Upstream only routes underscore-prefixed (`_`) methods to
`ExtensionMethodHandler`. Cursor sends `cursor/task`, so upstream returns
`method not found` and Cursor's subagent turn ends with no feedback.

The fork widens the seam **for inbound client requests only**:
`ClientSideConnection.handleWithExtensions` now falls through to the client's
`HandleExtensionMethod` for any method that is not a known stable method, as long
as it is a real inbound request. This is scoped deliberately:

- Only the **client** side is widened; the agent side still requires `_`.
- Only **requests** are delegated; vendor **notifications** are not routed to the
  extension handler.
- **Outbound** `CallExtension` / `NotifyExtension` still reject non-underscore
  names via `validateExtensionMethodName`.

Supporting pieces:

- `ClientSideConnection.isKnownMethod(method)` is generated into `client_gen.go`
  by `cmd/generate` (see `cmd/generate/internal/emit/dispatch.go`). It is
  reproducible by `make version` and must not be hand-edited.
- `isInboundRequest(ctx)` gates the fallback to genuine inbound requests.

Files: `extensions.go`, `connection.go`, `client_gen.go`,
`cmd/generate/internal/emit/dispatch.go`. Tests:
`TestExtensionMethods_*` in `acp_test.go`.

### 2. Legacy agent compatibility shims

Hand-written in `types_legacy.go` (with `types_legacy_test.go`):

- `LegacyModels` / `LegacyModelInfo`: read-only parsing of the pre-`v0.13.5`
  top-level `models` payload.
- `LegacyAgentMethodSessionSetModel` (`"session/set_model"`),
  `UnstableSetSessionModelRequest` / `UnstableSetSessionModelResponse`, and
  `ClientSideConnection.UnstableSetSessionModel(...)`: legacy model-selection
  wire method for unmigrated agents.
- Compatibility type aliases: `AuthMethodId`, `MessageId`, `TerminalId`,
  `DeleteSessionRequest`, `DeleteSessionResponse`.

### 3. Bounded inbound notification queue

In `connection.go`:

- `WithMaxQueuedNotifications(n)`: `ConnectionOption` to cap the inbound
  notification queue on both `NewClientSideConnection` and
  `NewAgentSideConnection`.
- `ErrNotificationQueueOverflow`: exported sentinel returned when the queue
  overflows, so the connection fails fast instead of growing unbounded.

Tests: `TestConnectionFailsFastOnNotificationQueueOverflow*` in `acp_test.go`.

### 4. Large JSON-RPC lines and peer-disconnect signal

In `connection.go` / `errors.go`:

- The inbound receive loop uses a chunked line reader with a 64 MiB per-frame
  cap (upstream's `bufio.Scanner` path capped frames at 10 MiB, which dropped the
  connection when an agent emitted a large `session/update`, for example a big
  diff).
- `ErrPeerDisconnected` is exported and request errors wrap it, so callers can
  use `errors.Is(err, ErrPeerDisconnected)`.

### 5. Schema 1.20 bindings and generator compatibility

- The stable and unstable Go bindings are regenerated from ACP schema 1.20.0
  (see `schema/version`, `sdk/version`).
- Stable `DeleteSession` replaces `UnstableDeleteSession`; the fork keeps
  `DeleteSessionRequest` / `DeleteSessionResponse` aliases in `types_legacy.go`
  for callers still on the unstable names.
- The generator was taught the fork's compatibility fields, so a plain
  `make version` reproduces them (see below).

## Code generation is self-contained

`cmd/generate` emits every fork-specific addition, so `make version` (or
`go run ./cmd/generate`) reproduces the committed generated files with no drift:

- `ClientSideConnection.isKnownMethod` in `client_gen.go`
  (`cmd/generate/internal/emit/dispatch.go`).
- The three `LegacyModels *LegacyModels` fields on `NewSessionResponse`,
  `LoadSessionResponse`, and `UnstableForkSessionResponse`
  (`needsLegacyModelsField` in `cmd/generate/internal/emit/types.go`).

Do not hand-edit generated `_gen.go` files. If you need a new fork field or
helper in a generated file, add it to the generator so it survives the next
regenerate.

## Upgrading from upstream

1. Rebase the fork branch onto the new upstream tag.
1. Run `make version` to regenerate bindings, then `make fmt`.
1. Confirm `git status` is clean after regenerate (no drift). If a fork field or
   helper disappeared, port it into `cmd/generate` rather than re-adding it by
   hand.
1. Re-run `go test ./...` and confirm the `TestExtensionMethods_*`,
   `TestLegacy*`, and `TestConnectionFailsFastOnNotificationQueueOverflow*`
   tests still pass.
1. Re-point Kandev's `apps/backend/go.mod` `replace` at the new commit.
