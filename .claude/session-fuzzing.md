# Session: Fuzzing Implementation

## Overview

Go native fuzzing (Go 1.18+) for parser components to discover edge cases, crashes, and security issues.

## Go Fuzzing Basics

- Fuzz functions: `FuzzXxx(f *testing.F)` in `*_test.go` files
- Seed corpus: `f.Add(...)` with interesting inputs
- Fuzz target: `f.Fuzz(func(t *testing.T, input string) { ... })`
- Run: `go test -fuzz=FuzzXxx -fuzztime=30s ./pkg/parser`
- **Type limitation**: Only primitives (`string`, `[]byte`, `int*`, `uint*`, `float*`, `bool`) - no structs

## Fuzz Targets (by risk)

| Priority | Target                     | File                                                | Risk                                               |
|:---------|:---------------------------|:----------------------------------------------------|:---------------------------------------------------|
| 1        | `ParseGitCommand()`        | `pkg/parser/git_fuzz_test.go`                       | Hand-written string manipulation, index arithmetic |
| 2        | `BashParser.Parse()`       | `pkg/parser/bash_fuzz_test.go`                      | Wraps mvdan.cc/sh, custom heredoc extraction       |
| 3        | `mdtable.Parse()`          | `pkg/mdtable/parser_fuzz_test.go`                   | Regex-based, ReDoS risk, manual `splitByPipe()`    |
| 4        | `JSONParser.Parse()`       | `internal/parser/json_fuzz_test.go`                 | Standard library, low risk                         |
| 5        | `PatternDetector.Detect()` | `internal/validators/secrets/detector_fuzz_test.go` | 25+ regex patterns, `getPosition()` offset calc    |

## Implementation Details

### Git Parser Fuzz Encoding

Since Go fuzzing doesn't support `[]string`, use tab-separated encoding:

```go
f.Add("git\tcommit\t-sS\t-m\tmessage")  // name\targ1\targ2...

f.Fuzz(func(t *testing.T, input string) {
    parts := strings.Split(input, "\t")
    if len(parts) == 0 {
        return
    }
    cmd := Command{Name: parts[0], Args: parts[1:]}
    result, err := ParseGitCommand(cmd)
    if err == nil {
        // Exercise all methods - should not panic
        _ = result.HasFlag("-s")
        _ = result.ExtractCommitMessage()
        // ...
    }
})
```

### Package Placement

Follow existing test patterns in the codebase:

- `pkg/` packages: Use `package xxx_test` suffix (external test package)
- `internal/` packages: Use `package xxx` (same package) for access to unexported constructors

### Fuzz Function Parameter Naming

When fuzz callback doesn't use `t *testing.T`, rename to `_` to satisfy linters:

```go
// Wrong - triggers revive unused-parameter lint
f.Fuzz(func(t *testing.T, input string) { ... })

// Correct - when t is unused
f.Fuzz(func(_ *testing.T, input string) { ... })
```

### Nil Check Pattern

When checking for nil and accessing fields, must return after the nil check:

```go
// Wrong - staticcheck SA5011 possible nil pointer dereference
if result == nil {
    t.Error("nil result")
}
_ = result.Tables  // SA5011: possible nil pointer dereference

// Correct
if result == nil {
    t.Error("nil result")
    return
}
_ = result.Tables
```

### Corpus Storage

Go fuzzer stores interesting inputs in `testdata/fuzz/<FuzzFunctionName>/`. Commit these for reproducibility.

## Taskfile Tasks

```bash
# Run all fuzz tests (10s each, suitable for CI)
task test:fuzz

# Run specific fuzz test with default 60s duration
task test:fuzz:git
task test:fuzz:bash
task test:fuzz:mdtable
task test:fuzz:json
task test:fuzz:secrets

# Run with custom duration
FUZZ_TIME=5m task test:fuzz:git
FUZZ_TIME=1m task test:fuzz
```

## Lint Issues Encountered

| Issue                   | File                  | Fix                                                 |
|:------------------------|:----------------------|:----------------------------------------------------|
| golines (line too long) | `json_fuzz_test.go`   | Split long `f.Add()` into multiple calls            |
| revive unused-parameter | All fuzz tests        | Rename `t *testing.T` to `_ *testing.T` when unused |
| staticcheck SA5011      | `parser_fuzz_test.go` | Add `return` after nil check                        |

## Testing Infrastructure Stats

- 55 test files total (50 unit + 5 fuzz)
- 17 generated mock files via `mockgen`
- 19 testscript `.txtar` integration tests
- Benchmark tests in `internal/validators/git/runner_benchmark_test.go`

## Best Practices

1. **Seed with real inputs**: Add corpus entries from existing unit tests
2. **Exercise all methods**: Call all public methods on parsed result to catch panics
3. **Check invariants**: Verify line/column numbers are ≥1, required fields are non-empty
4. **Handle parse errors gracefully**: Only validate results when `err == nil`
5. **Multiple event types**: For parsers with modes, test all (e.g., `JSONParser` with different `EventType`s)

## Future Improvements

- Add corpus files to git for regression testing
- Run extended fuzzing in CI nightly job
- Add coverage-guided fuzzing metrics
- Consider property-based testing with `rapid` for more complex invariants
