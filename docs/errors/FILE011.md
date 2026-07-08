# FILE011: Inline comment not allowed

## Error

Code being written or edited to a **source file** contains an in-body comment
that is not an allowed form (see below). By default klaudiush runs in **strict**
mode: every comment inside source code is blocked unless it is a task marker, a
machine directive, a doc comment on an exported declaration, or carries an
exception token.

## Why this matters

LLM-generated code tends to narrate itself — restating *what* a line does, or
padding a one-line decision with several lines of rationalization. Well-named
identifiers already communicate the "what", and load-bearing "why" explanations
are rare. Blocking inline comments by default pushes toward self-explanatory
code and keeps the few genuinely-needed explanations explicit and reviewable.

## How to fix

Remove the comment and let the code speak, or rename identifiers so the intent
is obvious:

```go
// Instead of:
count := 0 // holds the running total

// Fix: name it well and drop the comment
runningTotal := 0
```

If a comment is genuinely load-bearing (documents a non-obvious invariant),
append an exception token so it is allowed:

```go
if err != nil {
    return true // EXC:FILE011:unreadable-store-assumes-a-tracker-exists
}
```

## Allowed (not flagged)

- **Task and annotation markers**: `TODO`, `FIXME`, `HACK`, `XXX`, `BUG`, `WARNING`, `NOTE`, `OPTIMIZE`, `REVIEW`, `DEPRECATED`, and `@annotations`.
- **Doc comments** directly above an exported/public declaration (Go exported func/type/const/var, JS/TS `export`, Python public `def`/`class`). A blank line between the comment and the declaration breaks this exemption; comments on **unexported** declarations are not exempt.
- **Machine directives**: shebangs, Go compiler directives (build constraints, code generation), cgo directives, legacy build tags, character-encoding cookies, and the type/lint/coverage suppression comments recognised by language tooling.
- **Exception tokens**: any comment containing `EXC:<CODE>:<reason>`.
- **All comments in non-source files**: config, markup, data and shell files (`.toml`, `.yaml`, `.json`, `.md`, `.ini`, `.env`, `.sh`, `Makefile`, `Dockerfile`, ...) use the lenient pattern-based behaviour instead.

## Modes

- **strict** (default): blocks all in-body comments in source files except the allowed forms above.
- **filler**: blocks only comments matching a filler pattern — a verb-first restatement (initialize, loop, return, configure, handle, parse, encode, ...) or a "This function/method/... does/is/handles/..." restatement. Legitimate "why" comments are allowed. This is the pre-1.36 behaviour and is also used for non-source files regardless of mode.

## Configuration

```toml
[validators.file.ai_comments]
enabled = true
mode = "strict"   # "strict" (default) or "filler"
patterns = []     # custom filler-mode patterns (overrides defaults when set)
```

Restore the old lenient behaviour:

```toml
[validators.file.ai_comments]
mode = "filler"
```

Disable the validator entirely:

```toml
[validators.file.ai_comments]
enabled = false
```

## Hook output

When this error is triggered, klaudiush writes JSON to stdout:

**permissionDecisionReason** (shown to Claude):
`[FILE011] Inline comments are not allowed — write self-explanatory code instead. ...`

**systemMessage** (shown to user):
Formatted error with fix hint and reference URL.

**additionalContext** (behavioral guidance):
`Automated klaudiush validation check. Fix the reported errors and retry the same command.`

## Related

- [FILE010](FILE010.md) - linter ignore directives
