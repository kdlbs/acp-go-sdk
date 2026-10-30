<a href="https://agentclientprotocol.com/" >
  <img alt="Agent Client Protocol" src="https://zed.dev/img/acp/banner-dark.webp">
</a>

# ACP Go SDK

Go library for the Agent Client Protocol (ACP) - a standardized communication protocol
between code editors and AI‑powered coding agents.

Learn more about the protocol itself at <https://agentclientprotocol.com>.

## Versioning

SDK releases and ACP schema artifacts have independent version numbers:

- `sdk/version` contains the next Go SDK release version. SDK releases use tags
  such as `v0.14.0`.
- `schema/version` selects the published ACP schema artifact used to generate the
  bindings. Schema assets are downloaded from releases such as
  `schema-v1.28.0`.
- The root `version` file is a generated-schema stamp. It matches
  `schema/version`; it is not the SDK release version.

Run `make update-schema SCHEMA_VERSION=1.28.0` to update generated protocol bindings.
Run `make release VERSION=0.14.0` to prepare an SDK release without changing the
selected ACP schema.

## Installation

<!-- `$ printf 'go get github.com/coder/acp-go-sdk@v%s\n' "$(cat sdk/version)"` as bash -->

```bash
go get github.com/coder/acp-go-sdk@v0.14.0
```

## Get Started

### Understand the Protocol

Start by reading the [official ACP documentation](https://agentclientprotocol.com)
to understand the core concepts and protocol specification.

### Try the Examples

The [examples directory](https://github.com/coder/acp-go-sdk/tree/main/example)
contains simple implementations of both Agents and Clients in Go.
You can run them from your terminal or connect to external ACP agents.

- `go run ./example/agent` starts a minimal ACP agent over stdio.
- `go run ./example/claude-code` demonstrates bridging to Claude Code.
- `go run ./example/client` connects to a running agent and streams a sample turn.
- `go run ./example/gemini` bridges to the Gemini CLI in ACP mode (flags: -model, -sandbox, -debug, -gemini /path/to/gemini).

You can watch the interaction by running `go run ./example/client` locally.

### Explore the API

Browse the Go package docs on pkg.go.dev for detailed API documentation:

- <https://pkg.go.dev/github.com/coder/acp-go-sdk>

If you're building an [Agent](https://agentclientprotocol.com/protocol/overview#agent):

- Implement the `acp.Agent` interface (and optionally `acp.AgentLoader` for `session/load`).
- Create a connection with `acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)`.
- Send updates and make client requests using the returned connection.

If you're building a [Client](https://agentclientprotocol.com/protocol/overview#client):

- Implement the `acp.Client` interface (and optionally `acp.ClientTerminal` for
  terminal features).
- Launch or connect to your Agent process (stdio), then create a connection with
  `acp.NewClientSideConnection(client, stdin, stdout)`.
- Call `Initialize`, `NewSession`, and `Prompt` to run a turn and stream updates.

Connections accept options for transport-level behavior. For example,
`acp.WithMaxQueuedNotifications(n)` sets the per-connection capacity of the
inbound notification queue, while leaving outbound notifications and requests
unchanged. Values less than or equal to zero use the default capacity.

Helper constructors are provided to reduce boilerplate when working with union types:

- Content blocks: `acp.TextBlock`, `acp.ImageBlock`, `acp.AudioBlock`,
  `acp.ResourceLinkBlock`, `acp.ResourceBlock`.
- Tool content: `acp.ToolContent`, `acp.ToolDiffContent`, `acp.ToolTerminalRef`.
- Utility: `acp.Ptr[T]` for pointer fields in request/update structs.

### Protocol features in schema 1.28.0

The SDK supports ACP protocol v1 with stable and experimental schema inputs.
Protocol v2 remains a separate draft.

Clients advertise support during initialization:

```go
capabilities := acp.ClientCapabilities{
	Session: &acp.ClientSessionCapabilities{
		Notices:    &acp.NoticeCapabilities{},
		Compaction: &acp.CompactionCapabilities{},
	},
	Subagents: &acp.SubagentCapabilities{},
}
```

Agents send notices and compaction events only after the client advertises
the corresponding capability. Clients receive these events through `SessionUpdate`:

- `Notice` contains a severity, title, and optional description.
- `CompactionUpdate` tracks progress by `compactionId`.
- `CompactionSummaryChunk` appends content to the compaction summary.

Agent helpers include `UpdateNotice`, `UpdateCompaction`, and
`UpdateCompactionSummary`. The agent and client examples demonstrate these events.

#### Experimental subagents and messages

Clients advertise `Subagents` before agents expose child sessions.
An agent announces each child on its immediate parent session before it sends
child traffic or messages that identify the child.

```go
update := acp.UpdateSubagent("child-session")
update.SubagentUpdate.Title = acp.OptionalValue("Reviewer")
update.SubagentUpdate.Cwd = acp.OptionalValue("/workspace/review")
update.SubagentUpdate.State = acp.OptionalValue(acp.NewStateUpdateRunning())
```

Child events use the same connection. Clients do not need to load or subscribe
to the child session. `UpdateSessionMessage` and `UpdateSessionMessageChunk`
describe messages between sessions. Message upserts carry participant IDs and
references. Chunks carry content and a message ID.

Compaction, subagent, and message patches use `Optional[T]`:

- A zero value omits the field and keeps the stored value.
- `OptionalValue(value)` replaces the stored value.
- `OptionalNull[T]()` clears the stored value.
- `IsSet`, `IsNull`, and `Get` expose the decoded field state.

An empty slice replaces a list with an empty list. A `null` value clears it.
Subagent state is a complete snapshot. An idle child remains available for reuse.
Child mutations require explicit capabilities in that child's association.

#### Experimental system prompts

Agents advertise `AgentCapabilities.SessionCapabilities.SystemPrompt`.
Clients can then supply `SystemPrompt` on new, load, resume, and fork requests.
Append mode extends the agent's instructions. Override mode requires an
additional advertised capability.

```go
prompt := acp.SystemPromptText(acp.SystemPromptModeAppend, "Review changes before editing.")
request := acp.NewSessionRequest{
	Cwd:          "/workspace",
	McpServers:   []acp.McpServer{},
	SystemPrompt: &prompt,
}
```

The experimental fork API uses `UnstableSystemPrompt` and its corresponding
content types. Agents must reject unsupported system prompts.

#### Elicitation and MCP migration

Elicitation, terminal authentication, and tool names are stable in this schema.
`WithStartName` and `WithUpdateName` set programmatic tool names.

Clients implement the optional `ClientElicitation` interface and advertise
`ClientCapabilities.Elicitation` for structured forms or URL input.
The base `Client` interface has no new required methods.
The old `Unstable*Elicitation*` types, constructors, and connection methods remain
available as deprecated aliases or wrappers. Dispatch also accepts legacy handlers.

Experimental MCP-over-ACP now uses request-scoped messages:

- `UnstableMessageMcpRequest` identifies a declared server with `ServerId` and
  an active inner request with `RequestId`.
- `AgentSideConnection.UnstableMessageMcp` sends requests to the client.
- `ClientSideConnection.UnstableMessageMcp` sends notifications to the agent.
- Responses contain either `Result` or `Error`. A result can contain JSON `null`.
- Opaque MCP payload numbers use `json.Number` to preserve precision.
- `mcp/connect`, `mcp/disconnect`, and `connectionId` no longer apply.

Next Edit Suggestions, their document notifications, and `env_var` authentication
are absent from the new upstream schema. Their experimental Go APIs are removed.
The existing legacy model compatibility API remains available.

Schema 1.28 decoding silently drops malformed session configuration options,
including entries with a missing or empty `name`.
An invalid model option can therefore surface later as unavailable model selection.

### Extension methods

ACP supports **extension methods** for custom JSON-RPC methods whose names start with `_`.
Use them to add functionality without conflicting with future ACP versions.

#### Handling inbound extension methods

Implement `acp.ExtensionMethodHandler` on your Agent or Client. Your handler will be
invoked for any incoming method starting with `_`.

```go
// HandleExtensionMethod handles ACP extension methods (names starting with "_").
func (a MyAgent) HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "_example.com/hello":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return map[string]any{"greeting": "hello " + p.Name}, nil
	default:
		return nil, acp.NewMethodNotFound(method)
	}
}
```

> Note: Per the ACP spec, unknown extension notifications should be ignored.
> This SDK suppresses noisy logs for unhandled extension notifications that return
> “Method not found”.

#### Calling extension methods

From either side, use `CallExtension` / `NotifyExtension` on the connection.

```go
raw, err := conn.CallExtension(ctx, "_example.com/hello", map[string]any{"name": "world"})
if err != nil {
	return err
}

var resp struct {
	Greeting string `json:"greeting"`
}
if err := json.Unmarshal(raw, &resp); err != nil {
	return err
}

if err := conn.NotifyExtension(ctx, "_example.com/progress", map[string]any{"pct": 50}); err != nil {
	return err
}
```

#### Advertising extension support via `_meta`

ACP uses the `_meta` field inside capability objects as the negotiation/advertising
surface for extensions.

- Client -> Agent: `InitializeRequest.ClientCapabilities.Meta`
- Agent -> Client: `InitializeResponse.AgentCapabilities.Meta`

Keys `traceparent`, `tracestate`, and `baggage` are reserved in `_meta` for W3C trace
context/OpenTelemetry compatibility.

### Study a Production Implementation

For a complete, production‑ready integration, see the
[Gemini CLI Agent](https://github.com/google-gemini/gemini-cli) which exposes an
ACP interface. The Go example client `example/gemini` demonstrates connecting
to it via stdio.

## Resources

- [Go package docs](https://pkg.go.dev/github.com/coder/acp-go-sdk)
- [Examples (Go)](https://github.com/coder/acp-go-sdk/tree/main/example)
- [Protocol Documentation](https://agentclientprotocol.com)
- [Agent Client Protocol GitHub Repository](https://github.com/agentclientprotocol/agent-client-protocol)

## License

Apache 2.0. See [LICENSE](./LICENSE).
