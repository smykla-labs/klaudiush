# FILE011: Filler comment detected

## Error

Code being written or edited contains a comment that opens with an action
verb narrating *what* the next line does (e.g., `// Initialize the counter`,
`// Loop through items`, `// Return the result`, `// Configure the client`).

## Why this matters

Filler comments restate *what* the code does instead of *why* it does it.
Well-named identifiers already communicate the "what"; a comment repeating it
adds noise and rots the moment the code changes underneath it. This pattern is
common in LLM-generated code. Idiomatic doc comments (which open with the
identifier name) and "why" comments (which open with a reason) are unaffected.

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

A comment is flagged when it opens (behind `//` or `#`, optionally after
`the`/`a`/`an`) with a code-restating verb: initialize, set, get, loop,
iterate, check, return, create, make, build, call, invoke, execute, run,
increment/decrement, add/append/insert/push, remove/delete/clear/pop,
update/modify/change, handle, process, parse/format/convert, encode/decode,
marshal/unmarshal, compute/calculate, assign/declare/define, configure,
register, open/close/read/write, load/save/store, fetch/send/receive,
wait/start/stop, print/log/emit, render/draw, filter/sort/find/search/count,
validate/verify/ensure, and similar. Also caught: "This function/method/...
does/is/handles/...".

## Allowed (not flagged)

- Task and annotation markers: `// TODO: ...`, `// FIXME ...`, `// HACK`,
  `// XXX`, `// BUG`, `// OPTIMIZE`, `// REVIEW`, `// DEPRECATED`, `@annotations`.
- Doc comments directly above an exported/public declaration (Go exported
  func/type/const/var, JS/TS `export`, Python `def`/`class`) — a blank line
  between the comment and the declaration breaks this exemption.
- Any comment that does not open with a restating verb (e.g. a "why" comment).

## Configuration

Enable it and optionally override the pattern list:

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
