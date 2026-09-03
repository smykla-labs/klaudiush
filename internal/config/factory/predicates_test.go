package factory

import (
	"testing"

	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

func TestBeforeToolOrProviderAfterToolPredicateMatchesGeminiAfterTool(t *testing.T) {
	predicate := beforeToolOrProviderAfterToolPredicate()

	if !predicate(&hook.Context{
		Provider: hook.ProviderGemini,
		Event:    hook.CanonicalEventAfterTool,
	}) {
		t.Fatal("expected Gemini AfterTool to match post-action predicate")
	}
}

func TestBeforeToolOrProviderAfterToolPredicateDoesNotMatchClaudePostTool(t *testing.T) {
	predicate := beforeToolOrProviderAfterToolPredicate()

	if predicate(&hook.Context{
		Provider: hook.ProviderClaude,
		Event:    hook.CanonicalEventAfterTool,
	}) {
		t.Fatal("did not expect Claude post-tool events to match post-action predicate")
	}
}

func TestLifecycleEventPredicateMatchesPreCompress(t *testing.T) {
	predicate := lifecycleEventPredicate()

	if !predicate(&hook.Context{
		Provider: hook.ProviderGemini,
		Event:    hook.CanonicalEventPreCompress,
	}) {
		t.Fatal("expected PreCompress to match lifecycle predicate")
	}
}

// opencode reports the tool arguments on its after-tool event, so file
// validators can still inspect what was written. Excluding it would leave a
// rule scoped to tool.execute.after silently never running.
func TestBeforeToolOrProviderAfterToolPredicateMatchesOpenCodeAfterTool(t *testing.T) {
	predicate := beforeToolOrProviderAfterToolPredicate()

	if !predicate(&hook.Context{
		Provider: hook.ProviderOpenCode,
		Event:    hook.CanonicalEventAfterTool,
	}) {
		t.Fatal("expected opencode AfterTool to match post-action predicate")
	}
}

func TestBeforeToolOrProviderAfterToolPredicateMatchesOpenCodeBeforeTool(t *testing.T) {
	predicate := beforeToolOrProviderAfterToolPredicate()

	if !predicate(&hook.Context{
		Provider: hook.ProviderOpenCode,
		Event:    hook.CanonicalEventBeforeTool,
	}) {
		t.Fatal("expected opencode BeforeTool to match post-action predicate")
	}
}

// chat.message is offered as a rule event filter, so the rule engine has to be
// dispatched for it.
func TestLifecycleEventPredicateMatchesUserPromptSubmit(t *testing.T) {
	predicate := lifecycleEventPredicate()

	if !predicate(&hook.Context{
		Provider: hook.ProviderOpenCode,
		Event:    hook.CanonicalEventUserPromptSubmit,
	}) {
		t.Fatal("expected UserPromptSubmit to match lifecycle predicate")
	}
}
