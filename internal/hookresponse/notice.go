package hookresponse

import (
	"strings"

	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

// BuildNotice creates a minimal response carrying only a user-visible message
// in systemMessage. No permission decision is set, and the AI never sees it.
func BuildNotice(hookCtx *hook.Context, msg string) any {
	if hookCtx != nil && hookCtx.Provider == hook.ProviderCodex {
		return &CodexCommandResponse{
			Continue:      true,
			SystemMessage: msg,
		}
	}

	if hookCtx != nil && hookCtx.Provider == hook.ProviderGemini {
		return &GeminiCommandResponse{
			SystemMessage: msg,
		}
	}

	return &HookResponse{
		SystemMessage: msg,
	}
}

// AppendNotice appends a user-visible message to an existing response's
// systemMessage field, separated by a blank line.
func AppendNotice(resp any, msg string) {
	switch r := resp.(type) {
	case *HookResponse:
		r.SystemMessage = joinNotice(r.SystemMessage, msg)
	case *CodexCommandResponse:
		r.SystemMessage = joinNotice(r.SystemMessage, msg)
	case *GeminiCommandResponse:
		r.SystemMessage = joinNotice(r.SystemMessage, msg)
	case *ElicitationHookResponse:
		r.SystemMessage = joinNotice(r.SystemMessage, msg)
	}
}

// IsEmpty reports whether a built response carries nothing to write.
// Builders return typed nil pointers, which never compare equal to a nil
// interface, so callers cannot test the interface value directly.
func IsEmpty(resp any) bool {
	switch r := resp.(type) {
	case nil:
		return true
	case *HookResponse:
		return r == nil
	case *CodexCommandResponse:
		return r == nil
	case *GeminiCommandResponse:
		return r == nil
	case *ElicitationHookResponse:
		return r == nil
	default:
		return false
	}
}

// joinNotice separates messages with a single blank line, skipping empty parts.
func joinNotice(existing, msg string) string {
	if existing == "" {
		return msg
	}

	if msg == "" {
		return existing
	}

	return strings.TrimRight(existing, "\n") + "\n\n" + msg
}
