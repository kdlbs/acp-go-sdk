# Pin the v2 schema and add wire types

```yaml
id: 01-schema-and-wire-types
status: complete
wave: 1
depends_on: []
plan: plan.md
requirements:
  - REQ-SDK-ACP-V2-001
  - REQ-SDK-ACP-V2-002
  - REQ-SDK-ACP-V2-003
  - REQ-SDK-ACP-V2-004
acceptance_criteria:
  - AC-SDK-ACP-V2-001.1
  - AC-SDK-ACP-V2-002.1
  - AC-SDK-ACP-V2-002.3
  - AC-SDK-ACP-V2-003.2
  - AC-SDK-ACP-V2-004.1
  - AC-SDK-ACP-V2-004.2
system_design: system-design.md
```

Implement the data types the Preview client will use. Keep the code and
fixtures specific to wire version 2.

## Scope

- Save the pinned upstream `meta.json` and `schema.json` in `schema/v2/`.
  Record the tag, commit, and hashes from the [system design](system-design.md).
- Add the method constants and typed request, response, and callback payloads
  for the client surface in the design. Verify the v2 names and discriminators.
- Model the inbound update fields that Kandev will use. Keep future variants
  and optional extension objects available as raw JSON.

| Update family | Required wire surface |
| --- | --- |
| Foreground and replay | `state_update`, including state, stop reason, and background items. `replay_complete`. |
| Messages | Whole and chunk updates for user messages, agent messages, and thoughts. Preserve `messageId`. |
| Tools and display terminals | `tool_call_update`, `tool_call_content_chunk`, `terminal_update`, and `terminal_output_chunk`. Preserve entity IDs and content types. |
| Subagents | `subagent_update`, `session_message`, and `session_message_chunk`. Preserve parent and child identity. |
| Setup and reporting | Config options, commands, session info, plans, usage, notices, and compaction updates. |

Use separate types where v2 changes field names or semantics. Represent
uncommon nested content as typed discriminators with raw payloads when
Kandev needs only the type and data. This work order ends at the codec.

## Acceptance

1. Pinned schema files match their recorded SHA-256 hashes. The SDK's v1
   schema and generated bindings still pass their existing tests.
1. Wire fixtures verify initialization, auth, setup, config options, prompt,
   resume, close, cancel, and callbacks. Include stdio and HTTP MCP servers.
1. Tests distinguish omitted, null, empty, and populated patch values. Unknown
   enum strings and well-formed future updates survive decoding and encoding.

## Verification

From the SDK repository root:

```bash
go test -trimpath ./experimental/v2 -run '^TestWire_' -count=1
go test -trimpath ./...
git diff --check
```

Add table tests for the following wire cases. Use the `TestWire_` prefix.

- `configId` with the select or boolean discriminator.
- `methodId` on an auth method and `messageId` on a prompt result.
- `replayFrom: {"type":"start"}` and the same resume call with no replay.
- A message upsert with empty content followed by a chunk.
- A tool patch with an absent field, a null field, and a concrete value.
- A future update and metadata with a numeric value larger than 2⁵³.
- Required fields on a permission request and the elicitation scope rules.

## Files and dependencies

Likely paths include `schema/v2/`, `experimental/v2/types.go`, focused
update files, and package-local `testdata/`. Use Go 1.21 standard library
JSON and the root SDK's optional-field type where suitable.

The schema release and this package provide the fixtures. The work can run
without a second repository or a live agent.

## Results

Both files in `schema/v2/` match the SHA-256 hashes in the system design.
The wire fixtures and the root SDK suite passed. The case for a select
configuration option covers both wire discriminators.
The option uses `type: "select"`. A request to set its value uses
`type: "id"`. [Pinned Preview schema](../../../schema/v2/README.md)

- `go test -trimpath ./experimental/v2 -run '^TestWire_' -count=1` passed.
- `go test -trimpath ./...` passed.
- `git diff --check` passed.
