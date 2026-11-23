# Implementation plan for the ACP v2 preview client

```yaml
created: 2026-11-23
status: draft
requirements: requirements.md
system_design: system-design.md
```

Build a small, typed v2 client in this repository for Kandev's pilot.
Start from the existing `github.com/coder/acp-go-sdk` module. The original
investigation used SDK revision `ec6d622b1999`.

## Start here

1. Read the [requirements](requirements.md) and [system design](system-design.md).
1. Implement [01 — Schema and wire types](task-01-schema-and-wire-types.md).
1. Implement [02 — Client connection and interoperability](task-02-client-connection.md).
1. Report the tested SDK revision and public API to the Kandev consumer.

Work order 02 depends on 01. Both orders use fixtures and paired pipes.
They can run from this checkout without provider accounts or a Kandev server.

## Delivery boundary

The SDK delivers the Preview wire API, typed fallback classification, and
offline interoperability tests. The Kandev repository owns the feature flag,
agent restart, debugger, adapter, and product qualification.

The root ACP v1 API remains the entry point for existing SDK consumers.
The Preview import path is `github.com/coder/acp-go-sdk/experimental/v2`.
This work package covers the client role over the existing stdio transport.
Schema generator changes, a general v2 agent API, extra transports, and
vendor extensions have separate scope.

## Wire evidence

The Kandev qualification spike ran Codex ACP `2.8.0` and `2.9.0`, and
Claude ACP `0.109.0` and `0.109.1`. Each completed an ACP v2 turn and replay
against synthetic model output. Both protocol versions restored the saved
conversation. The fixtures cover both valid prompt orders.

- Codex reported `running`, then returned the prompt acknowledgement.
- Claude returned the prompt acknowledgement, then reported `running`.

Both agents sent output and an idle update with `stopReason: "end_turn"`.
The SDK tests keep the prompt reply and idle update separate. The same
probe found a version 2 reply with v1 `agentInfo` and `agentCapabilities`.
Add that shape as a third-party compatibility fixture.

Use sanitized, minimal fixtures with synthetic IDs and paths. Representative
examples and the pinned upstream source are in the [system design](system-design.md).
Raw provider traces are optional implementation evidence.

## Verification

From the SDK repository root after both work orders:

```bash
go test -trimpath ./...
go test -trimpath -race ./experimental/v2/... -count=1
GOTOOLCHAIN=go1.21.13 go test -trimpath ./...
make test
git diff --check
```

Use the repository toolchain for `make fmt`. Confirm that the existing
generator tests and examples pass as part of `make test`.

## Report to the consumer

- Supply the tested revision and Preview import path.
- List the client constructor, handlers, and typed legacy-selection error.
- Record the schema tag and offline test results.
- Give the Kandev integrator the revision for its module replacement.

The Kandev integrator owns the dependency pin. A local module override can
test both checkouts before a published revision is available.
