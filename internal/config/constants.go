package config

// Conventional commit type names used in default validator configuration.
const (
	commitTypeBuild    = "build"
	commitTypeChore    = "chore"
	commitTypeCI       = "ci"
	commitTypeDocs     = "docs"
	commitTypeFeat     = "feat"
	commitTypeFix      = "fix"
	commitTypePerf     = "perf"
	commitTypeRefactor = "refactor"
	commitTypeRevert   = "revert"
	commitTypeStyle    = "style"
	commitTypeTest     = "test"
)

// Config section and validator path segment names.
const (
	sectionGlobal       = "global"
	sectionValidators   = "validators"
	sectionRules        = "rules"
	sectionExceptions   = "exceptions"
	sectionPatterns     = "patterns"
	sectionCrashDump    = "crash_dump"
	sectionBypassPerms  = "bypass_permissions"
	sectionGit          = "git"
	sectionFile         = "file"
	sectionNotification = "notification"
	sectionSecrets      = "secrets"
	sectionShell        = "shell"

	validatorCommit       = "commit"
	validatorPush         = "push"
	validatorFetch        = "fetch"
	validatorAdd          = "add"
	validatorPR           = "pr"
	validatorBranch       = "branch"
	validatorNoVerify     = "no_verify"
	validatorMerge        = "merge"
	validatorMessage      = "message"
	validatorMarkdown     = "markdown"
	validatorShellScript  = "shellscript"
	validatorTerraform    = "terraform"
	validatorWorkflow     = "workflow"
	validatorGofumpt      = "gofumpt"
	validatorPython       = "python"
	validatorJavaScript   = "javascript"
	validatorRust         = "rust"
	validatorLinterIgnore = "linter_ignore"
	validatorBell         = "bell"
	validatorBacktick     = "backtick"
)

// Common config map keys.
const (
	keyEnabled      = "enabled"
	keySeverity     = "severity"
	keyTimeout      = "timeout"
	keyValidTypes   = "valid_types"
	keyContextLines = "context_lines"
)

// Severity string values.
const (
	severityError   = "error"
	severityWarning = "warning"
)

// Markdown lint rule identifiers disabled by default.
const (
	mdRuleLineLength = "MD013"
	mdRuleBareURLs   = "MD034"
	mdRuleFirstLine  = "MD041"
)

// Style and preference values.
const (
	valueAuto   = "auto"
	valueCustom = "custom"
)

// branchMain is the default protected branch name.
const branchMain = "main"
