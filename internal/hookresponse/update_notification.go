package hookresponse

import (
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

// BuildUpdateNotification creates a minimal response containing only
// the update notification in systemMessage. No permission decision is set.
func BuildUpdateNotification(hookCtx *hook.Context, msg string) any {
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

// AppendUpdateNotification appends an update notification to an existing
// response's systemMessage field, separated by a blank line.
func AppendUpdateNotification(resp any, msg string) {
	switch r := resp.(type) {
	case *HookResponse:
		r.SystemMessage = r.SystemMessage + "\n\n" + msg
	case *CodexCommandResponse:
		r.SystemMessage = r.SystemMessage + "\n\n" + msg
	case *GeminiCommandResponse:
		r.SystemMessage = r.SystemMessage + "\n\n" + msg
	case *ElicitationHookResponse:
		r.SystemMessage = r.SystemMessage + "\n\n" + msg
	}
}
