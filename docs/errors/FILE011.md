# FILE011: Filler comment detected

## Error

Code being written or edited contains a comment that only restates the line
it annotates (e.g., `// Initialize the counter`, `// Loop through items`,
`// Return the result`).

## Why this matters

Filler comments narrate *what* the code does instead of *why* it does it.
Well-named identifiers already communicate the "what"; a comment repeating it
adds noise without adding information, and rots the moment the code changes
underneath it. This pattern is common in LLM-generated code and is disabled
by default — enable it if you want klaudiush to push back on it.

## How to fix

Remove the comment, or replace it with one that explains a non-obvious
reason:

```go
// Instead of:
// Initialize the counter
count := 0

// Fix: remove it, or explain the why
count := 0 // starts at zero to match the 1-indexed API offset
```

## Detected patterns

klaudiush flags common throat-clearing phrases behind `//` or `#`:

- Initialize/initializing
- Loop/iterate through/over
- Check if
- Return(s/ing) the
- Create(s) a new
- Set(s) the
- Increment/decrement
- Call(s) the
- Get(s) the
- "This function/method/class does/is/handles/will"

## Configuration

Disabled by default. Enable it and optionally override the pattern list:

```toml
[validators.file.ai_comments]
enabled = true
patterns = []   # custom patterns (overrides defaults when set)
```

Disable the validator:

```toml
[validators.file.ai_comments]
enabled = false
```

## Hook output

When this error is triggered, klaudiush writes JSON to stdout:

**permissionDecisionReason** (shown to Claude):
`[FILE011] Filler comments that only restate the code are not allowed. Remove the comment or replace it with one that explains why, not what`

**systemMessage** (shown to user):
Formatted error with fix hint and reference URL.

**additionalContext** (behavioral guidance):
`Automated klaudiush validation check. Fix the reported errors and retry the same command.`

## Related

- [FILE010](FILE010.md) - linter ignore directives
