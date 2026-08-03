// Package bypass decides how klaudiush behaves when a session runs without
// approval prompts (Claude --dangerously-skip-permissions, Codex
// --dangerously-bypass-approvals-and-sandbox, Gemini --yolo).
package bypass

import (
	"slices"

	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

// Policy answers bypass questions for a hook context.
// A nil Policy enforces validation and stays silent.
type Policy struct {
	cfg *config.BypassPermissionsConfig
}

// NewPolicy creates a Policy from configuration. A nil config keeps the
// defaults: validate everything, notify the user once per session.
func NewPolicy(cfg *config.BypassPermissionsConfig) *Policy {
	return &Policy{cfg: cfg}
}

// ModeActive reports whether the context runs under a bypass permission mode.
func (p *Policy) ModeActive(hookCtx *hook.Context) bool {
	if p == nil || hookCtx == nil || hookCtx.PermissionMode == "" {
		return false
	}

	return slices.Contains(p.modes(), hookCtx.PermissionMode)
}

// SkipValidation reports whether validation should be skipped for this context.
func (p *Policy) SkipValidation(hookCtx *hook.Context) bool {
	if !p.ModeActive(hookCtx) {
		return false
	}

	return p.cfg.IsSkipValidation()
}

// NotifyEnabled reports whether the user-only notice should be emitted.
func (p *Policy) NotifyEnabled() bool {
	if p == nil {
		return false
	}

	return p.cfg.IsNotifyEnabled()
}

// modes returns the permission modes treated as bypass modes.
func (p *Policy) modes() []string {
	if p == nil || len(p.cfg.GetModes()) == 0 {
		return hook.DefaultBypassModes()
	}

	return p.cfg.GetModes()
}
