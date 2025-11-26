# Session: Plugin System Implementation

Implementation of external validator plugins for klaudiush, enabling extensibility through Go plugins, exec plugins, and (future) gRPC plugins.

## Architecture

### Public API (`pkg/plugin/api.go`)

Plugin authors implement the `Plugin` interface:

```go
type Plugin interface {
    Info() Info
    Validate(req *ValidateRequest) *ValidateResponse
}
```

**Request**: Contains hook context (event type, tool name, command, file path, content, config)

**Response**: Validation result (passed, should_block, message, error_code, fix_hint, doc_link)

**Helper Functions**:

- `PassResponse()`, `FailResponse()`, `WarnResponse()`
- `FailWithCode()`, `WarnWithCode()` for structured errors
- `AddDetail()` for additional context

### Internal Architecture

**Plugin Interface** (`internal/plugin/plugin.go`):

- Internal wrapper around public plugin interface
- Adds context support for timeouts and cancellation
- Includes `Close()` for resource cleanup

**Loader Interface** (`internal/plugin/loader.go`):

- Abstraction for loading plugins from different sources
- `Load(cfg) (Plugin, error)` creates plugin instances
- `Close()` for loader cleanup

**Implementations**:

1. **GoLoader**: Native Go plugins (.so files)
   - Uses Go's `plugin` package
   - Looks up exported "Plugin" symbol
   - No unloading (Go limitation), Close() is no-op
   - Runs synchronously in-process

2. **ExecLoader**: Subprocess plugins (JSON over stdin/stdout)
   - Protocol: `--info` flag returns metadata, stdin/stdout for validation
   - Uses `internal/exec.CommandRunner` for execution
   - Timeouts enforced via context
   - Full JSON marshaling of requests/responses

3. **GRPCLoader**: Deferred for future implementation
   - Would use protobuf-defined service
   - Connection pooling for performance
   - Persistent connections unlike exec plugins

### Registry & Predicate Matching

**Registry** (`internal/plugin/registry.go`):

- Manages plugin loading and lifecycle
- Creates loaders for each plugin type
- Stores loaded plugins with their predicates
- Provides `GetValidators(hookCtx)` for context-based matching

**PredicateMatcher**:

- Filters plugins based on hook context
- Four filter types:
  - **Event types**: PreToolUse, PostToolUse, Notification
  - **Tool types**: Bash, Write, Edit, Grep, etc.
  - **File patterns**: Glob patterns for file tools (e.g., `*.go`, `**/*.tf`)
  - **Command patterns**: Regex for bash commands (e.g., `^git commit`)
- Refactored into helper methods to reduce cognitive complexity
- Empty filter = match all (allows catch-all plugins)

### Validator Integration

**ValidatorAdapter** (`internal/plugin/adapter.go`):

- Adapts `Plugin` to `Validator` interface
- Converts `hook.Context` → `plugin.ValidateRequest`
- Converts `plugin.ValidateResponse` → `validator.Result`
- Maps error codes, fix hints, doc links
- Assigns category for parallel execution:
  - Go plugins: `CategoryCPU` (in-process)
  - Exec plugins: `CategoryIO` (subprocess overhead)

**PluginValidatorFactory** (`internal/config/factory/plugin_factory.go`):

- Creates plugin registry and loads all configured plugins
- Returns `PluginRegistryValidator` that delegates to registry
- Registered with catch-all `EventTypePreToolUse` predicate
- Individual plugins do fine-grained matching via their predicates

## Configuration Schema

```toml
[plugins]
enabled = true
directory = "~/.klaudiush/plugins"
default_timeout = "5s"

[[plugins.plugins]]
name = "company-rules"
type = "go"  # or "exec" or "grpc"
enabled = true
path = "~/.klaudiush/plugins/company-rules.so"
timeout = "10s"

[plugins.plugins.predicate]
event_types = ["PreToolUse"]
tool_types = ["Write", "Edit"]
file_patterns = ["*.go", "**/*.rs"]
command_patterns = []  # regex patterns for bash commands

[plugins.plugins.config]
# Plugin-specific config passed to plugin via ValidateRequest.Config
max_line_length = 120
require_copyright_header = true
```

### Configuration Features

- **Per-plugin enable/disable**: Each plugin has `enabled` flag (default: true)
- **Plugin-specific config**: Arbitrary key-value map passed to plugin
- **Timeout control**: Per-plugin or global default
- **Flexible predicates**: Fine-grained control over when plugins run

## Error Handling

**Static Errors** (err113 compliance):

```go
var (
    ErrPluginInfoFailed     = errors.New("plugin --info exited with non-zero code")
    ErrPluginExecFailed     = errors.New("plugin execution failed with non-zero code")
    ErrPluginNilResponse    = errors.New("plugin returned nil response")
)
```

**Wrapping**: Uses `errors.Wrapf()` to add context while preserving static errors

## Constants

Extracted magic numbers for mnd compliance:

```go
const (
    defaultExecPluginTimeout = 5 * time.Second
    defaultPluginTimeout     = 5 * time.Second
    defaultRegistryTimeout   = 10 * time.Second
)
```

## Linting Fixes

- **err113**: Static error constants with `errors.Wrapf()` for context
- **gocognit**: Refactored `Matches()` into helper methods
- **golines**: Split long function signatures across multiple lines
- **ireturn**: Added `//nolint:ireturn` for interface returns (required by design)
- **mnd**: Extracted magic numbers to named constants
- **modernize**: Used `slices.Contains()` instead of loops
- **revive**: Removed unused receiver or renamed to `_`
- **wastedassign**: Removed unused assignments
- **gofumpt**: Proper formatting (var blocks, etc.)

## Plugin Categorization for Parallel Execution

- **Go plugins**: `CategoryCPU` (pure computation, in-process)
- **Exec plugins**: `CategoryIO` (subprocess overhead, I/O-bound)
- **Future gRPC**: Likely `CategoryIO` (network overhead)

Categorization allows dispatcher to use appropriate worker pools for optimal concurrency.

## Future Work (Deferred)

- **gRPC plugin loader**: Requires protobuf setup, service definition, code generation
- **Unit tests**: Test each loader in isolation with mocks
- **Integration tests**: End-to-end plugin loading and validation
- **Plugin examples**: Sample plugins demonstrating best practices
- **Plugin development guide**: Documentation for plugin authors

## Files Created

- `pkg/plugin/api.go`: Public API for plugin authors
- `pkg/config/plugin.go`: Configuration schema
- `internal/plugin/plugin.go`: Internal plugin interface
- `internal/plugin/loader.go`: Loader interface
- `internal/plugin/go_loader.go`: Go plugin loader
- `internal/plugin/exec_loader.go`: Exec plugin loader
- `internal/plugin/registry.go`: Plugin registry and predicate matching
- `internal/plugin/adapter.go`: Validator adapter for plugins
- `internal/config/factory/plugin_factory.go`: Factory integration

## Files Modified

- `pkg/config/config.go`: Added `Plugins` field to root config
- `internal/config/factory/factory.go`: Integrated plugin factory into validator creation
