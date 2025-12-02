# Parallel Validator Execution Architecture

Category-based parallel execution with semaphore pools to maximize validation throughput while preventing resource contention.

## Core Design Philosophy

**Category-Based Pooling**: Validators grouped by workload characteristics (CPU-bound, I/O-bound, Git operations). Each category has dedicated worker pool sized for optimal concurrency.

**Git Serialization**: Git operations run serially (pool size = 1) to prevent index lock contention. Git's `.git/index.lock` causes failures when multiple processes access repository simultaneously.

**Semaphore-Based Concurrency**: Use `golang.org/x/sync/semaphore` for bounded concurrency per category. Cleaner than worker pool pattern for this use case.

**Race-Free Shared State**: Mutex protects shared result slices. Go race detector (`-race`) verifies thread safety in tests.

## Validator Category System

Three categories based on resource usage patterns:

```go
// internal/validator/category.go
type ValidatorCategory int

const (
    CategoryCPU ValidatorCategory = iota  // Default: Pure computation
    CategoryIO                             // External processes (shellcheck, terraform)
    CategoryGit                            // Git operations (serialized)
)
```

### Category Characteristics

| Category    | Pool Size  | Examples                                | Rationale                                        |
|:------------|:-----------|:----------------------------------------|:-------------------------------------------------|
| CategoryCPU | NumCPU     | Regex matching, commit message parsing  | CPU-bound, benefits from parallelism up to cores |
| CategoryIO  | NumCPU × 2 | shellcheck, terraform, actionlint, gRPC | I/O-bound, can overlap while waiting on I/O      |
| CategoryGit | 1 (serial) | GetStagedFiles, GetCurrentBranch        | Prevents `.git/index.lock` contention            |

**Why NumCPU × 2 for I/O**: I/O operations spend most time waiting. 2× oversubscription allows other work while waiting on external processes.

**Why Serial for Git**: Git uses `.git/index.lock` file for mutual exclusion. Concurrent git operations cause "fatal: Unable to create '.git/index.lock': File exists" errors.

## Executor Pattern

Two executor implementations with identical interface:

```go
// internal/dispatcher/executor.go
type Executor interface {
    Execute(ctx context.Context, hookCtx *hook.Context, validators []validator.Validator) *Result
}

// SequentialExecutor - Default, runs validators in order
type SequentialExecutor struct{}

// ParallelExecutor - Semaphore-based pools per category
type ParallelExecutor struct {
    cpuSem *semaphore.Weighted   // Size: runtime.NumCPU()
    ioSem  *semaphore.Weighted   // Size: runtime.NumCPU() * 2
    gitSem *semaphore.Weighted   // Size: 1
}
```

### Sequential Executor

Simple loop, no concurrency:

```go
func (e *SequentialExecutor) Execute(ctx context.Context, hookCtx *hook.Context, validators []validator.Validator) *Result {
    result := &Result{}
    for _, v := range validators {
        vResult := v.Validate(hookCtx)
        result.AddValidationResult(v.Name(), vResult)
    }
    return result
}
```

**When to Use**: Testing, debugging, or when parallel execution causes issues.

### Parallel Executor

Semaphore pools with mutex-protected results:

```go
func (e *ParallelExecutor) Execute(ctx context.Context, hookCtx *hook.Context, validators []validator.Validator) *Result {
    result := &Result{}
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, v := range validators {
        wg.Add(1)
        go func(v validator.Validator) {
            defer wg.Done()

            // Acquire semaphore based on category
            sem := e.getSemaphoreForCategory(v.Category())
            if err := sem.Acquire(ctx, 1); err != nil {
                return  // Context cancelled
            }
            defer sem.Release(1)

            // Execute validation
            vResult := v.Validate(hookCtx)

            // Append to shared results (mutex protected)
            mu.Lock()
            result.AddValidationResult(v.Name(), vResult)
            mu.Unlock()
        }(v)
    }

    wg.Wait()
    return result
}
```

**Critical Details**:

1. **Acquire before goroutine work**: Semaphore acquired inside goroutine prevents goroutine leak if context cancelled
2. **Defer Release**: Ensures semaphore released even if Validate() panics
3. **Mutex on shared state**: `result.AddValidationResult()` not thread-safe
4. **WaitGroup**: Ensures all validators complete before returning

## Validator Integration

Validators implement `Category()` method to declare workload type:

```go
// internal/validators/file/shellcheck_validator.go
type ShellCheckValidator struct {
    validator.BaseValidator
    checker linters.ShellChecker
}

func (v *ShellCheckValidator) Category() validator.ValidatorCategory {
    return validator.CategoryIO  // Spawns external shellcheck process
}
```

**Default Category**: If validator doesn't implement `Category()`, `BaseValidator` returns `CategoryCPU`.

### Category Assignment Examples

```go
// CPU-bound validators
type CommitMessageValidator struct { ... }
func (*CommitMessageValidator) Category() validator.ValidatorCategory {
    return validator.CategoryCPU  // Regex matching, string parsing
}

// I/O-bound validators
type TerraformValidator struct { ... }
func (*TerraformValidator) Category() validator.ValidatorCategory {
    return validator.CategoryIO  // Spawns terraform/tofu process
}

// Git validators
type AddValidator struct { ... }
func (*AddValidator) Category() validator.ValidatorCategory {
    return validator.CategoryGit  // Calls git operations
}
```

## Testing Concurrent Code

### Race Detection

Always run tests with race detector:

```bash
go test -race ./...
task test  # Includes -race flag
```

Race detector catches:

- Concurrent writes to shared variables
- Write during concurrent read
- Mutex missing on shared state

**Gotcha**: Race detector increases memory usage ~10× and slows tests ~2-10×. Don't use in production builds.

### Deadlock Prevention Tests

Use context timeout to detect deadlocks:

```go
func TestParallelExecutor_NoDeadlock(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    executor := NewParallelExecutor()
    result := executor.Execute(ctx, hookCtx, validators)

    // Test passes if completes before timeout
    // Would hang indefinitely if deadlocked
    assert.NotNil(t, result)
}
```

**Why 5 Seconds**: Long enough for legitimate execution, short enough to fail fast on deadlock.

### Early Termination Tests

Verify context cancellation stops execution:

```go
func TestParallelExecutor_ContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    // Cancel immediately
    cancel()

    executor := NewParallelExecutor()
    result := executor.Execute(ctx, hookCtx, validators)

    // Should return quickly without executing validators
    assert.NotNil(t, result)
}
```

### Concurrency Patterns Used

1. **`golang.org/x/sync/semaphore`**: Bounded concurrency per category
2. **`sync.Mutex`**: Protects shared result slice
3. **`sync.WaitGroup`**: Waits for all goroutines to complete
4. **`context.WithTimeout`**: Detects deadlocks in tests
5. **`context.WithCancel`**: Tests early termination

## Performance Characteristics

### Sequential vs Parallel

**Scenario**: 10 validators (3 CPU, 5 I/O, 2 Git), each takes 100ms

**Sequential**:

- Total time: 10 validators × 100ms = **1000ms**

**Parallel** (4-core machine):

- CPU validators: 3 validators / 4 cores = **100ms** (all run in parallel)
- I/O validators: 5 validators / 8 workers = **100ms** (all run in parallel)
- Git validators: 2 validators × 100ms = **200ms** (serialized)
- Total time: **max(100, 100, 200) = 200ms** (5× faster)

**Real-world speedup**: Typically 3-6× depending on validator mix and I/O latency.

### Semaphore Overhead

Semaphore acquire/release adds ~1-5μs overhead per validator. Negligible compared to validation time (typically milliseconds).

## Go 1.22+ Integer Range Loop

Modern Go syntax for counting loops:

```go
// MODERN (Go 1.22+) - Preferred
for i := range 10 {
    // i goes from 0 to 9
}

// OLD STYLE - Avoid
for i := 0; i < 10; i++ {
    // Same behavior
}
```

**Why Preferred**: Cleaner syntax, less error-prone (no forgotten increment, no off-by-one in condition).

## Common Pitfalls

1. **Forgetting Category() method**: Validator defaults to CategoryCPU. I/O-bound validators in CPU pool waste parallelism.

2. **Using CategoryCPU for I/O operations**: External processes (shellcheck, terraform) spend time waiting. Use CategoryIO for 2× concurrency.

3. **Not serializing Git operations**: Concurrent git commands cause `.git/index.lock` errors. Always use CategoryGit.

4. **Missing mutex on shared results**: Concurrent `result.AddValidationResult()` causes data races. Always protect with mutex.

5. **Acquiring semaphore outside goroutine**: If context cancelled before goroutine starts, semaphore never released → deadlock.

6. **Not using race detector in tests**: Data races may not manifest immediately. Always test with `go test -race`.

7. **Long timeouts in deadlock tests**: Use 5-10 second timeouts to fail fast. Longer timeouts slow down test suite when deadlock occurs.

8. **Not handling context cancellation**: Semaphore `Acquire()` can fail if context cancelled. Check error before validation.

9. **Assuming sequential execution order**: Parallel execution makes order non-deterministic. Don't depend on execution order in validators.

10. **Testing only with sequential executor**: Concurrency bugs only appear with ParallelExecutor. Test both executors.
