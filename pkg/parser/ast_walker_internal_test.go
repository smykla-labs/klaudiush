package parser

import (
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

// These cover defensive guards that are unreachable through normal parsing
// (a CallExpr always has a name, a statement always has a command) but are kept
// for safety.

func TestCallExprOfNilStatement(t *testing.T) {
	if got := callExprOf(nil); got != nil {
		t.Fatalf("callExprOf(nil) = %v, want nil", got)
	}
}

func TestLiteralCommandOutputNoArgs(t *testing.T) {
	if _, ok := literalCommandOutput(&syntax.CallExpr{}); ok {
		t.Fatal("literalCommandOutput with no args should not be capturable")
	}
}

func TestIsLiteralWordNil(t *testing.T) {
	if isLiteralWord(nil) {
		t.Fatal("isLiteralWord(nil) = true, want false")
	}
}

func TestParamExpToStringNil(t *testing.T) {
	if got := paramExpToString(nil); got != "" {
		t.Fatalf("paramExpToString(nil) = %q, want empty", got)
	}
}

func TestPrintfOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{"no args", nil, "", false},
		{"bare literal", []string{"fix: x"}, "fix: x", true},
		{"single %s", []string{"%s", "fix: x"}, "fix: x", true},
		{"%s with trailing newline trimmed", []string{`%s\n`, "fix: x"}, "fix: x", true},
		{
			"multi %s keeps the blank line",
			[]string{`%s\n\n%s\n`, "title", "body"},
			"title\n\nbody",
			true,
		},
		{"literal percent", []string{`100%% done`}, "100% done", true},
		{"tab escape", []string{`a\tb`}, "a\tb", true},
		{"unsupported directive", []string{"%d", "42"}, "", false},
		{"too many args (recycling declined)", []string{`%s\n`, "a", "b"}, "", false},
		{"too few args", []string{"%s %s", "a"}, "", false},
		{"dangling percent", []string{"done%"}, "", false},
		{"unknown escape", []string{`a\x41`}, "", false},
		{"trailing backslash", []string{`a\`}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := printfOutput(tt.args)
			if ok != tt.ok {
				t.Fatalf("printfOutput(%q) ok = %v, want %v", tt.args, ok, tt.ok)
			}

			if got != tt.want {
				t.Fatalf("printfOutput(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
