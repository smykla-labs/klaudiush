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

| Priority | Target                     | Risk                                               |
|:---------|:---------------------------|:---------------------------------------------------|
| 1        | `ParseGitCommand()`        | Hand-written string manipulation, index arithmetic |
| 2        | `BashParser.Parse()`       | Wraps mvdan.cc/sh, custom heredoc extraction       |
| 3        | `mdtable.Parse()`          | Regex-based, ReDoS risk, manual `splitByPipe()`    |
| 4        | `JSONParser.Parse()`       | Standard library, low risk                         |
| 5        | `PatternDetector.Detect()` | 25+ regex patterns, `getPosition()` offset calc    |

## Key Implementation Details

### Git Parser Fuzz Encoding

Since Go fuzzing doesn't support `[]string`, use tab-separated encoding:

```go
f.Add("git\tcommit\t-sS\t-m\tmessage")  // name\targ1\targ2...

f.Fuzz(func(t *testing.T, input string) {
    parts := strings.Split(input, "\t")
    cmd := Command{Name: parts[0], Args: parts[1:]}
    // ...
})
```

### Package Placement

- `pkg/` packages: Use same package (no `_test` suffix) to access internal types
- `internal/` packages: Same - need access to unexported constructors

### Corpus Storage

Go fuzzer stores interesting inputs in `testdata/fuzz/<FuzzFunctionName>/`. Commit these for reproducibility.

## Testing Infrastructure Stats

- 50 test files, ~13,285 lines of test code
- 17 generated mock files via `mockgen`
- 19 testscript `.txtar` integration tests
- Benchmark tests in `internal/validators/git/runner_benchmark_test.go`

## Progress Tracking

Implementation tracked in `tmp/fuzzing/`:

- `PLAN.md` - Full implementation plan with code examples
- `PROGRESS.md` - Checklist and session log

## Taskfile Tasks (to add)

```yaml
test:fuzz:       # All fuzz tests, 10s each (CI)
test:fuzz:git:   # Git parser, 60s
test:fuzz:bash:  # Bash parser, 60s
test:fuzz:mdtable:  # Markdown table, 60s
test:fuzz:json:  # JSON parser, 60s
test:fuzz:secrets:  # Secrets detector, 60s
```

## Files to Create

- `pkg/parser/git_fuzz_test.go`
- `pkg/parser/bash_fuzz_test.go`
- `pkg/mdtable/parser_fuzz_test.go`
- `internal/parser/json_fuzz_test.go`
- `internal/validators/secrets/detector_fuzz_test.go`
