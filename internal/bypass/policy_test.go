package bypass

import (
	"strings"
	"testing"

	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

func TestPolicyModeActive(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.BypassPermissionsConfig
		hookCtx *hook.Context
		want    bool
	}{
		{
			name:    "claude bypass mode",
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeBypass},
			want:    true,
		},
		{
			name:    "codex danger full access",
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeDangerFullAccess},
			want:    true,
		},
		{
			name:    "gemini yolo",
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeYolo},
			want:    true,
		},
		{
			name:    "ordinary mode",
			hookCtx: &hook.Context{PermissionMode: "acceptEdits"},
			want:    false,
		},
		{
			name:    "empty mode",
			hookCtx: &hook.Context{},
			want:    false,
		},
		{
			name:    "nil context",
			hookCtx: nil,
			want:    false,
		},
		{
			name:    "custom modes replace the defaults",
			cfg:     &config.BypassPermissionsConfig{Modes: []string{"dontAsk"}},
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeBypass},
			want:    false,
		},
		{
			name:    "custom modes match",
			cfg:     &config.BypassPermissionsConfig{Modes: []string{"dontAsk"}},
			hookCtx: &hook.Context{PermissionMode: "dontAsk"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPolicy(tt.cfg).ModeActive(tt.hookCtx); got != tt.want {
				t.Errorf("ModeActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicySkipValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.BypassPermissionsConfig
		hookCtx *hook.Context
		want    bool
	}{
		{
			name:    "defaults keep validating",
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeBypass},
			want:    false,
		},
		{
			name:    "opt-out skips in bypass mode",
			cfg:     &config.BypassPermissionsConfig{SkipValidation: new(true)},
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeBypass},
			want:    true,
		},
		{
			name:    "opt-out does not skip ordinary modes",
			cfg:     &config.BypassPermissionsConfig{SkipValidation: new(true)},
			hookCtx: &hook.Context{PermissionMode: "default"},
			want:    false,
		},
		{
			name: "expired opt-out validates again",
			cfg: &config.BypassPermissionsConfig{
				SkipValidation: new(true),
				ExpiresAt:      "2020-01-01T00:00:00Z",
			},
			hookCtx: &hook.Context{PermissionMode: hook.PermissionModeBypass},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPolicy(tt.cfg).SkipValidation(tt.hookCtx); got != tt.want {
				t.Errorf("SkipValidation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyNotifyEnabled(t *testing.T) {
	if !NewPolicy(nil).NotifyEnabled() {
		t.Error("NotifyEnabled() = false for default config, want true")
	}

	cfg := &config.BypassPermissionsConfig{Notify: new(false)}
	if NewPolicy(cfg).NotifyEnabled() {
		t.Error("NotifyEnabled() = true when notify is off, want false")
	}

	var nilPolicy *Policy
	if nilPolicy.NotifyEnabled() {
		t.Error("NotifyEnabled() = true for nil policy, want false")
	}

	if nilPolicy.SkipValidation(&hook.Context{PermissionMode: hook.PermissionModeBypass}) {
		t.Error("SkipValidation() = true for nil policy, want false")
	}
}

func TestNoticeMentionsModeAndCommands(t *testing.T) {
	notice := Notice(hook.PermissionModeBypass)

	for _, want := range []string{
		hook.PermissionModeBypass,
		"klaudiush bypass skip",
		"klaudiush bypass notify off",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("Notice() = %q, want it to contain %q", notice, want)
		}
	}
}
