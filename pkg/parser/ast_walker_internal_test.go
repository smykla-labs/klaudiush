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
