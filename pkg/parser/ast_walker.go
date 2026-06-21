package parser

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// astWalker walks the AST and extracts commands and file operations.
type astWalker struct {
	commands   []Command
	fileWrites []FileWrite
	currentDir string // Tracks the effective working directory from cd commands
	// stdinByCall maps a CallExpr to the content fed to its stdin (heredoc or
	// piped echo/printf). Populated when a Stmt or pipeline is visited, then
	// consumed when the corresponding CallExpr is extracted into a Command.
	stdinByCall map[*syntax.CallExpr]string
}

// visit is called for each node in the AST.
func (w *astWalker) visit(node syntax.Node) bool {
	switch n := node.(type) {
	case *syntax.BinaryCmd:
		w.extractPipedStdin(n)
	case *syntax.CallExpr:
		w.extractCommand(n)
	case *syntax.Stmt:
		w.extractRedirect(n)
	case *syntax.Subshell:
		// Subshells are handled recursively by syntax.Walk
		return true
	case *syntax.CmdSubst:
		// Command substitution is handled recursively
		return true
	}

	return true
}

// recordStdin associates stdin content with a CallExpr so it can be attached
// to the Command when that CallExpr is later extracted.
func (w *astWalker) recordStdin(call *syntax.CallExpr, content string) {
	if w.stdinByCall == nil {
		w.stdinByCall = make(map[*syntax.CallExpr]string)
	}

	w.stdinByCall[call] = content
}

// extractPipedStdin handles "producer | consumer" pipelines, capturing the
// producer's literal output (echo/printf) as the consumer's stdin. This lets
// validators inspect messages fed via "git commit -F -".
func (w *astWalker) extractPipedStdin(bin *syntax.BinaryCmd) {
	if bin.Op != syntax.Pipe && bin.Op != syntax.PipeAll {
		return
	}

	producer := callExprOf(bin.X)
	consumer := callExprOf(bin.Y)

	if producer == nil || consumer == nil {
		return
	}

	content, ok := literalCommandOutput(producer)
	if !ok {
		return
	}

	w.recordStdin(consumer, content)
}

// callExprOf returns the CallExpr a statement runs, or nil if the statement is
// not a simple command.
func callExprOf(stmt *syntax.Stmt) *syntax.CallExpr {
	if stmt == nil {
		return nil
	}

	if call, ok := stmt.Cmd.(*syntax.CallExpr); ok {
		return call
	}

	return nil
}

// literalCommandOutput returns the text a simple echo/printf command writes to
// stdout, when it can be determined from literal arguments alone.
func literalCommandOutput(call *syntax.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}

	name := wordToString(call.Args[0])
	if name != "echo" && name != "printf" {
		return "", false
	}

	// Only capture when every argument is strictly literal. Expansions
	// (parameters, command/arithmetic substitution, globs) cannot be
	// reproduced here, so capturing would yield a message different from what
	// the command actually emits.
	args, ok := literalArgs(call.Args[1:])
	if !ok {
		return "", false
	}

	if name == "echo" {
		return echoOutput(args)
	}

	return printfOutput(args) // name == "printf"
}

// literalArgs converts words to strings only when every word is strictly
// literal. It returns false as soon as any word contains a shell expansion.
func literalArgs(words []*syntax.Word) ([]string, bool) {
	args := make([]string, 0, len(words))

	for _, word := range words {
		if !isLiteralWord(word) {
			return nil, false
		}

		args = append(args, wordToString(word))
	}

	return args, true
}

// isLiteralWord reports whether a word consists solely of literal text - plain
// literals and single/double-quoted literals - with no shell expansions such as
// parameters, command/arithmetic substitution, or globbing.
func isLiteralWord(word *syntax.Word) bool {
	if word == nil {
		return false
	}

	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit, *syntax.SglQuoted:
			// Literal text, no expansion.
		case *syntax.DblQuoted:
			if !allLiteralParts(p.Parts) {
				return false
			}
		default:
			return false
		}
	}

	return true
}

// allLiteralParts reports whether every part is a plain literal (no expansions),
// as found inside a double-quoted string.
func allLiteralParts(parts []syntax.WordPart) bool {
	for _, p := range parts {
		if _, ok := p.(*syntax.Lit); !ok {
			return false
		}
	}

	return true
}

// echoOutput returns the literal text content of an "echo args..." call. Only
// the leading run of echo flags (-n, -e, -E and combinations) is stripped; once
// a non-flag word appears, every remaining argument is literal, even if it
// starts with "-". It returns false when -e is present, since that enables
// backslash-escape interpretation which this helper does not perform - capturing
// the raw text would mis-validate the message. This is a best-effort capture for
// validation: it joins the arguments with spaces and omits the trailing newline
// echo would normally emit (callers trim surrounding whitespace anyway).
func echoOutput(args []string) (string, bool) {
	i := 0

	for i < len(args) {
		flag, ok := echoFlag(args[i])
		if !ok {
			break
		}

		if strings.ContainsRune(flag, 'e') {
			return "", false
		}

		i++
	}

	return strings.Join(args[i:], " "), true
}

// echoFlag reports whether arg is an echo option like -n, -e, -E, or -ne and
// returns its letters (without the leading dash) when so.
func echoFlag(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}

	for _, c := range arg[1:] {
		if c != 'n' && c != 'e' && c != 'E' {
			return "", false
		}
	}

	return arg[1:], true
}

// printfOutput returns the literal text content of a simple "printf" call, for
// the forms commonly used to feed a commit message: a bare literal format with
// no directives, or "%s"/"%s\n" with a single string argument. This is a
// best-effort capture for validation: it returns the argument text itself and
// does not apply any newline implied by the format string (callers trim
// surrounding whitespace anyway).
func printfOutput(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}

	format := args[0]

	// "printf <literal>" with no format directives.
	if len(args) == 1 && !strings.Contains(format, "%") {
		return format, true
	}

	// "printf %s msg" / "printf '%s\n' msg" with a single argument.
	if len(args) == 2 && (format == "%s" || format == "%s\n" || format == `%s\n`) {
		return args[1], true
	}

	return "", false
}

// extractCommand extracts a command from a CallExpr node.
func (w *astWalker) extractCommand(call *syntax.CallExpr) {
	if len(call.Args) == 0 {
		return
	}

	// First word is the command name
	name := wordToString(call.Args[0])
	if name == "" {
		return
	}

	// Remaining words are arguments
	args := wordsToStrings(call.Args[1:])

	// Determine location
	loc := Location{
		Line:   call.Pos().Line(),
		Column: call.Pos().Col(),
	}

	// Determine command type (simple for now, enhanced later)
	cmdType := CmdTypeSimple

	cmd := Command{
		Name:             name,
		Args:             args,
		Location:         loc,
		Type:             cmdType,
		WorkingDirectory: w.currentDir,
		Stdin:            w.stdinByCall[call],
	}

	w.commands = append(w.commands, cmd)

	// Check if this is a cd command and update current directory
	if name == "cd" && len(args) > 0 {
		w.currentDir = args[0]
	}

	// Check if this is a file write command
	w.extractFileWriteCommand(cmd)
}

// redirInfo holds the output redirection and heredoc found on a statement.
type redirInfo struct {
	outputPath     string
	outputOp       WriteOp
	outputLoc      Location
	heredocContent string
	heredocLoc     Location
	hasOutput      bool
	hasHeredoc     bool
}

// collectRedirs gathers output redirection and heredoc details from a statement.
func collectRedirs(stmt *syntax.Stmt) redirInfo {
	var info redirInfo

	for _, redir := range stmt.Redirs {
		switch redir.Op {
		case syntax.RdrOut, syntax.AppOut:
			path := wordToString(redir.Word)
			if path == "" {
				continue
			}

			info.outputPath = path

			info.outputOp = WriteOpRedirect
			if redir.Op == syntax.AppOut {
				info.outputOp = WriteOpAppend
			}

			info.outputLoc = Location{Line: redir.Pos().Line(), Column: redir.Pos().Col()}
			info.hasOutput = true
		case syntax.Hdoc, syntax.DashHdoc:
			// Extract heredoc content from Hdoc field (may be empty).
			if redir.Hdoc != nil {
				info.heredocContent = wordToString(redir.Hdoc)
			}
			// Mark as heredoc even if content is empty.
			info.heredocLoc = Location{Line: redir.Pos().Line(), Column: redir.Pos().Col()}
			info.hasHeredoc = true
		default:
			// Other redirection operators are not relevant here.
		}
	}

	return info
}

// extractRedirect extracts file write operations and stdin content from redirections.
func (w *astWalker) extractRedirect(stmt *syntax.Stmt) {
	if stmt.Redirs == nil {
		return
	}

	info := collectRedirs(stmt)

	// A heredoc always feeds the command's stdin, regardless of any output
	// redirection on the same statement. Record it so validators can inspect
	// stdin-fed content (e.g. "git commit -F - <<EOF ... EOF >/dev/null").
	if info.hasHeredoc {
		if call := callExprOf(stmt); call != nil {
			w.recordStdin(call, info.heredocContent)
		}
	}

	switch {
	case info.hasOutput && info.hasHeredoc:
		// Output redirection combined with a heredoc. The heredoc body equals the
		// file bytes only when it is an overwrite (">", not ">>") and the command
		// copies stdin to stdout verbatim (cat). A transforming command such as
		// "grep foo > f <<EOF" writes filtered output, not the heredoc body, so
		// that content must not be treated as captured.
		captured := info.outputOp == WriteOpRedirect && copiesStdinVerbatim(callExprOf(stmt))
		w.fileWrites = append(w.fileWrites, FileWrite{
			Path:             info.outputPath,
			Operation:        WriteOpHeredoc,
			Content:          info.heredocContent,
			ContentCaptured:  captured,
			Location:         info.heredocLoc,
			WorkingDirectory: w.currentDir,
		})
	case info.hasOutput:
		// Just output redirection without heredoc. Content is intentionally not
		// captured here: it would flow to the dispatcher's synthetic file-write
		// validation and lint partial bytes (e.g. "echo 'x' > foo.go" tripping
		// gofumpt). Heredoc bodies are the only redirect content captured.
		w.fileWrites = append(w.fileWrites, FileWrite{
			Path:             info.outputPath,
			Operation:        info.outputOp,
			Location:         info.outputLoc,
			WorkingDirectory: w.currentDir,
		})
	}
}

// copiesStdinVerbatim reports whether the command copies its stdin to stdout
// unchanged, so that a heredoc fed to it becomes the exact redirected output.
// Only "cat" (reading stdin implicitly) or "cat -" (reading stdin once) qualify.
// "cat file", "cat -n", "cat - -" (stdin emitted more than once), and
// transforming commands (grep, sed, ...) do not.
func copiesStdinVerbatim(call *syntax.CallExpr) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}

	if wordToString(call.Args[0]) != "cat" {
		return false
	}

	switch rest := call.Args[1:]; len(rest) {
	case 0:
		return true // "cat"
	case 1:
		return wordToString(rest[0]) == "-" // "cat -"
	default:
		return false // "cat - -", "cat file", ...
	}
}

// extractFileWriteCommand detects file write commands (tee, cp, mv).
func (w *astWalker) extractFileWriteCommand(cmd Command) {
	op, targets := getFileWriteOperation(cmd)
	if op == WriteOpNone {
		return
	}

	for _, target := range targets {
		fw := FileWrite{
			Path:             target,
			Operation:        op,
			Source:           cmd.Name,
			Location:         cmd.Location,
			WorkingDirectory: cmd.WorkingDirectory,
		}

		w.fileWrites = append(w.fileWrites, fw)
	}
}

// getFileWriteOperation determines if a command writes to files.
func getFileWriteOperation(cmd Command) (WriteOp, []string) {
	switch cmd.Name {
	case "tee":
		// tee writes to all file arguments
		return WriteOpTee, extractTeeTargets(cmd.Args)

	case "cp", "copy":
		// cp writes to the last argument
		if len(cmd.Args) >= 2 { //nolint:mnd // Trivial check for minimum args (source + dest)
			return WriteOpCopy, []string{cmd.Args[len(cmd.Args)-1]}
		}

	case "mv", "move":
		// mv writes to the last argument
		if len(cmd.Args) >= 2 { //nolint:mnd // Trivial check for minimum args (source + dest)
			return WriteOpMove, []string{cmd.Args[len(cmd.Args)-1]}
		}
	}

	return WriteOpNone, nil
}

// extractTeeTargets extracts file targets from tee command arguments.
func extractTeeTargets(args []string) []string {
	targets := make([]string, 0)

	// Skip flags (starting with -)
	for _, arg := range args {
		if len(arg) > 0 && arg[0] != '-' {
			targets = append(targets, arg)
		}
	}

	return targets
}
