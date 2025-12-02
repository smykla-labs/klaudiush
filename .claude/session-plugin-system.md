# Plugin System Architecture

Extensible validator plugin system supporting Go (.so), exec (subprocess), and gRPC plugins with predicate-based matching and category-aware parallel execution.

## Core Design Philosophy

**Loader Pattern for Plugin Types**: Each plugin type (Go, exec, gRPC) has dedicated loader implementing common `Loader` interface. Allows adding new plugin types without modifying core system.

**Predicate-Based Matching**: Plugins declare when they should run via predicates (event types, tool types, file patterns, command regex). Registry matches plugins to hook context dynamically.

**Public API Stability**: Plugin authors use stable `pkg/plugin/api.go` interface. Internal implementation can evolve without breaking plugins.

**Category-Aware Execution**: Plugins categorized by resource usage (CPU vs I/O). Exec plugins use CategoryIO pool (2× CPUs), Go plugins use CategoryCPU pool (NumCPU workers).

**Static Error Constants**: All plugin errors use static error constants (err113 compliance) wrapped with context via `errors.Wrapf()`.

## Public Plugin API

Plugin authors implement single interface:

```go
// pkg/plugin/api.go
package pluginapi

type Plugin interface {
    Info() Info
    Validate(req *ValidateRequest) *ValidateResponse
}

type Info struct {
    Name        string
    Version     string
    Description string
}

type ValidateRequest struct {
    Context *HookContext
    Config  map[string]any  // Plugin-specific config from TOML
}

type HookContext struct {
    EventType string  // "PreToolUse", "PostToolUse", "Notification"
    ToolName  string  // "Bash", "Write", "Edit", "Grep"
    ToolInput struct {
        Command  string // Bash command
        FilePath string // Write/Edit/Read file path
        Content  string // Write/Edit content
    }
}

type ValidateResponse struct {
    Passed      bool
    ShouldBlock bool
    Message     string
    ErrorCode   string  // e.g., "COMPANY001"
    FixHint     string
    DocLink     string  // e.g., "https://company.com/rules/COMPANY001"
    Details     map[string]string
}
```

### Helper Functions

```go
// Quick response builders
func PassResponse() *ValidateResponse
func FailResponse(msg string) *ValidateResponse
func WarnResponse(msg string) *ValidateResponse

// With structured error codes
func FailWithCode(code, msg, fixHint, docLink string) *ValidateResponse
func WarnWithCode(code, msg, fixHint, docLink string) *ValidateResponse

// Add extra context
func (r *ValidateResponse) AddDetail(key, value string) *ValidateResponse
```

**Why Separate Helpers**: Plugin authors shouldn't need to remember struct field names or boolean combinations. Helpers enforce correct patterns.

## Internal Architecture

### Plugin Interface

Internal wrapper adds context support:

```go
// internal/plugin/plugin.go
type Plugin interface {
    Info() pluginapi.Info
    Validate(ctx context.Context, req pluginapi.ValidateRequest) (pluginapi.ValidateResponse, error)
    Close() error  // Resource cleanup
}
```

**Why Context**: Allows timeout and cancellation for hanging plugins. Public API stays simple (no context).

### Loader Interface

Abstraction for loading plugins from different sources:

```go
// internal/plugin/loader.go
type Loader interface {
    Load(cfg *config.PluginConfig) (Plugin, error)
    Close() error
}
```

Three implementations: `GoLoader`, `ExecLoader`, `GRPCLoader`.

## Plugin Type Implementations

### Go Loader (.so files)

Native Go plugins using `plugin` package:

```go
// internal/plugin/go_loader.go
type GoLoader struct{}

func (l *GoLoader) Load(cfg *config.PluginConfig) (Plugin, error) {
    // Open .so file
    p, err := plugin.Open(cfg.Path)
    if err != nil {
        return nil, errors.Wrapf(ErrPluginLoadFailed, "path: %s", cfg.Path)
    }

    // Lookup exported "Plugin" symbol
    sym, err := p.Lookup("Plugin")
    if err != nil {
        return nil, errors.Wrap(ErrPluginSymbolNotFound, "symbol: Plugin")
    }

    // Type assert to pluginapi.Plugin
    pluginImpl, ok := sym.(pluginapi.Plugin)
    if !ok {
        return nil, ErrPluginWrongType
    }

    return &goPluginAdapter{plugin: pluginImpl, timeout: cfg.Timeout}, nil
}
```

**Characteristics**:

- **Performance**: Fastest (in-process, no serialization)
- **Language**: Go only
- **Reloading**: Not possible (Go limitation - plugins can't be unloaded)
- **Category**: CategoryCPU (pure computation)
- **Use case**: Performance-critical validation, shared codebases

**Gotcha**: Go plugins require exact Go version match. Plugin compiled with Go 1.21.0 won't load in Go 1.21.1 binary.

### Exec Loader (subprocess)

JSON-based protocol over stdin/stdout:

```go
// internal/plugin/exec_loader.go
type ExecLoader struct {
    runner exec.CommandRunner
}

func (l *ExecLoader) Load(cfg *config.PluginConfig) (Plugin, error) {
    // Verify plugin exists and is executable
    if err := validateExecutable(cfg.Path); err != nil {
        return nil, err
    }

    // Test plugin with --info flag
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    result, err := l.runner.Run(ctx, cfg.Path, []string{"--info"})
    if err != nil {
        return nil, errors.Wrapf(ErrPluginInfoFailed, "path: %s", cfg.Path)
    }

    var info pluginapi.Info
    if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
        return nil, errors.Wrap(ErrPluginInfoInvalid, err.Error())
    }

    return &execPluginAdapter{
        path:    cfg.Path,
        info:    info,
        timeout: cfg.Timeout,
        runner:  l.runner,
    }, nil
}
```

**Protocol**:

1. **Info**: `plugin --info` returns JSON with name/version/description
2. **Validate**: JSON request on stdin, JSON response on stdout
3. **Exit codes**: 0 = success, non-zero = error

**Characteristics**:

- **Performance**: Slower (process spawn ~50-100ms overhead)
- **Language**: Any (shell, Python, Node.js, Ruby, etc.)
- **Reloading**: Yes (new process each invocation)
- **Category**: CategoryIO (subprocess overhead)
- **Use case**: Cross-language plugins, simple validators

### gRPC Loader

See `session-grpc-loader.md` for complete architecture. Key points:

- **Connection pooling**: Reuses connections across plugin instances
- **TLS security**: Auto-enabled for remote addresses
- **Category**: CategoryIO (network overhead)
- **Use case**: Long-running plugins with state, production deployments

## Predicate Matching System

Plugins declare when they run via predicates in configuration:

```toml
[plugins.plugins.predicate]
event_types = ["PreToolUse"]                    # Hook event types
tool_types = ["Write", "Edit"]                  # Tool names
file_patterns = ["*.go", "**/*.proto"]          # Glob patterns
command_patterns = ["^git commit", "^docker.*"] # Regex for Bash commands
```

### PredicateMatcher Implementation

```go
// internal/plugin/registry.go
type PredicateMatcher struct {
    eventTypes      []string
    toolTypes       []string
    filePatterns    []string
    commandPatterns []*regexp.Regexp
}

func (pm *PredicateMatcher) Matches(ctx *hook.Context) bool {
    if !pm.matchesEventType(ctx.EventType) {
        return false
    }
    if !pm.matchesToolType(ctx.ToolName) {
        return false
    }
    if !pm.matchesFilePattern(ctx) {
        return false
    }
    if !pm.matchesCommandPattern(ctx) {
        return false
    }
    return true
}
```

**Matching Rules**:

- **Empty list = match all**: Plugin with no event_types matches all events
- **AND logic**: All specified filters must match
- **Glob patterns**: Use `doublestar` for `**` support
- **Regex patterns**: Full regex power for command matching

**Refactoring**: Split `Matches()` into helper methods (`matchesEventType`, etc.) to reduce cognitive complexity below gocognit threshold.

## Validator Integration

### ValidatorAdapter

Adapts Plugin to Validator interface:

```go
// internal/plugin/adapter.go
type ValidatorAdapter struct {
    plugin   Plugin
    config   *config.PluginConfig
    category validator.ValidatorCategory
}

func (a *ValidatorAdapter) Validate(ctx *hook.Context) validator.Result {
    // Convert hook.Context → pluginapi.ValidateRequest
    req := pluginapi.ValidateRequest{
        Context: convertHookContext(ctx),
        Config:  a.config.Config,
    }

    // Call plugin with timeout
    timeoutCtx, cancel := context.WithTimeout(context.Background(), a.config.Timeout)
    defer cancel()

    resp, err := a.plugin.Validate(timeoutCtx, req)
    if err != nil {
        return validator.FailWithRef(
            validator.RefPluginError,
            fmt.Sprintf("Plugin %s failed: %v", a.config.Name, err),
        )
    }

    // Convert pluginapi.ValidateResponse → validator.Result
    result := validator.Result{
        Passed:      resp.Passed,
        ShouldBlock: resp.ShouldBlock,
        Message:     resp.Message,
    }

    // Use plugin's error documentation if provided
    if resp.DocLink != "" {
        result.Reference = validator.Reference(resp.DocLink)
    }
    result.FixHint = resp.FixHint

    return result
}

func (a *ValidatorAdapter) Category() validator.ValidatorCategory {
    return a.category
}
```

**Why Adapter**: Keeps plugin API simple (no validator imports). Adapter handles conversion and timeout enforcement.

### Plugin Registry

Central registry managing plugin lifecycle:

```go
// internal/plugin/registry.go
type Registry struct {
    loaders    map[config.PluginType]Loader
    validators []validatorWithPredicate  // Loaded plugins + predicates
}

type validatorWithPredicate struct {
    validator *ValidatorAdapter
    predicate *PredicateMatcher
}

func (r *Registry) GetValidators(ctx *hook.Context) []validator.Validator {
    var matched []validator.Validator
    for _, vp := range r.validators {
        if vp.predicate.Matches(ctx) {
            matched = append(matched, vp.validator)
        }
    }
    return matched
}
```

**Registry Lifecycle**:

1. **Load**: Create loaders for each plugin type (Go, exec, gRPC)
2. **Register**: Load each configured plugin, create adapter, store with predicate
3. **Match**: For each hook context, return validators matching predicates
4. **Close**: Close all loaders and plugins

## Configuration Schema

```toml
[plugins]
enabled = true                          # Global enable/disable
directory = "~/.klaudiush/plugins"      # Default plugin directory
default_timeout = "5s"                  # Timeout for plugins without explicit timeout

[[plugins.plugins]]
name = "company-rules"
type = "go"                            # "go", "exec", or "grpc"
enabled = true                         # Per-plugin enable (default: true)
path = "~/.klaudiush/plugins/company-rules.so"
timeout = "10s"                        # Override default timeout

[plugins.plugins.predicate]
event_types = ["PreToolUse"]           # When to run
tool_types = ["Write", "Edit"]        # Which tools
file_patterns = ["*.go", "**/*.rs"]   # Which files
command_patterns = ["^git commit"]     # Which commands (regex)

[plugins.plugins.config]
# Plugin-specific config (passed to plugin via ValidateRequest.Config)
max_line_length = 120
require_copyright_header = true
allowed_licenses = ["MIT", "Apache-2.0"]
```

**Configuration Features**:

- **Per-plugin enable/disable**: Quick way to temporarily disable plugin
- **Plugin-specific config**: Arbitrary TOML config passed to plugin
- **Timeout control**: Global default with per-plugin override
- **Flexible predicates**: Fine-grained control over execution

## Error Handling

### Static Error Constants

```go
// internal/plugin/errors.go
var (
    ErrPluginLoadFailed       = errors.New("failed to load plugin")
    ErrPluginSymbolNotFound   = errors.New("plugin symbol not found")
    ErrPluginWrongType        = errors.New("plugin symbol has wrong type")
    ErrPluginInfoFailed       = errors.New("plugin --info exited with non-zero code")
    ErrPluginExecFailed       = errors.New("plugin execution failed")
    ErrPluginNilResponse      = errors.New("plugin returned nil response")
)
```

**Why Static**: err113 linter requires static error constants for sentinel errors. Allows `errors.Is()` checks.

### Error Wrapping Pattern

```go
// BAD - Dynamic error message
return fmt.Errorf("failed to load plugin from %s: %w", path, err)

// GOOD - Static constant with wrapped context
return errors.Wrapf(ErrPluginLoadFailed, "path: %s: %w", path, err)
```

Context added via `Wrapf()` while preserving static error for `errors.Is()` checks.

## Linter Compliance Patterns

### err113: Static Errors

All errors must be static constants. Use `errors.Wrapf()` for context.

### gocognit: Cognitive Complexity

Split complex functions into helpers:

```go
// Before: Matches() had complexity 15
func (pm *PredicateMatcher) Matches(ctx *hook.Context) bool {
    // ... 50 lines of nested logic
}

// After: Split into helpers (complexity 4)
func (pm *PredicateMatcher) Matches(ctx *hook.Context) bool {
    return pm.matchesEventType(ctx.EventType) &&
           pm.matchesToolType(ctx.ToolName) &&
           pm.matchesFilePattern(ctx) &&
           pm.matchesCommandPattern(ctx)
}
```

### ireturn: Interface Returns

Loader `Load()` returns `Plugin` interface (required by design):

```go
//nolint:ireturn // Loader pattern requires interface return
func (l *GoLoader) Load(cfg *config.PluginConfig) (Plugin, error) {
    // ...
}
```

### mnd: Magic Number Detection

Extract constants:

```go
// BAD
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)

// GOOD
const defaultPluginTimeout = 5 * time.Second
ctx, cancel := context.WithTimeout(ctx, defaultPluginTimeout)
```

### modernize: Use Standard Library

```go
// OLD
for _, t := range pm.eventTypes {
    if t == ctx.EventType {
        return true
    }
}
return false

// MODERN (Go 1.21+)
return slices.Contains(pm.eventTypes, ctx.EventType)
```

## Common Pitfalls

1. **Not checking plugin enabled flag**: Always check `cfg.Enabled` before loading. Disabled plugins should be skipped.

2. **Missing timeout on plugin operations**: Plugins can hang. Always use `context.WithTimeout` for plugin calls.

3. **Forgetting Close() on loaders**: Resource leak if loaders not closed. Defer `Close()` after creating registry.

4. **Empty predicate matching logic**: Empty predicate list means "match all", not "match none". This is intentional (allows catch-all plugins).

5. **Not handling nil response**: Exec plugins can return nil if output parsing fails. Always check response != nil.

6. **Using dynamic errors**: err113 linter rejects dynamic errors. Use static constants with `errors.Wrapf()`.

7. **Go version mismatch**: Go plugins require exact version match. Plugin compiled with Go 1.21.0 won't load in 1.21.1.

8. **Not validating executable path**: Exec plugins must be executable. Check file exists and has execute bit before loading.

9. **Complex predicate matching in single function**: High cognitive complexity triggers gocognit. Extract helper methods.

10. **Assuming sequential execution**: Plugins run in parallel (category-based pools). Don't rely on execution order.