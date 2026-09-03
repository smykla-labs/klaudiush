package hook

import "testing"

// Rule authors match on event names, and a rule that can never fire is worse
// than a rejected one: it silently disables the policy it was written for. Every
// dotted opencode id offered to rule authors must therefore survive the
// normalize/display round trip the parser performs, and end up in EventNames().
func TestOpenCodeRuleEventNamesAreMatchable(t *testing.T) {
	// Mirrors the dotted opencode entries in pkg/config.ValidEventTypes.
	// Duplicated rather than imported because pkg/config imports pkg/hook.
	ruleEventNames := []string{
		"tool.execute.before",
		"tool.execute.after",
		"chat.message",
		"session.created",
		"session.idle",
		"session.compacting",
		"session.compacted",
		"permission.asked",
	}

	for _, eventName := range ruleEventNames {
		t.Run(eventName, func(t *testing.T) {
			canonical := NormalizeEventName(eventName)
			if canonical == CanonicalEventUnknown {
				t.Fatalf("%q does not normalize to a canonical event", eventName)
			}

			ctx := &Context{
				Provider: ProviderOpenCode,
				Event:    canonical,
				RawEventName: DisplayEventName(
					ProviderOpenCode,
					canonical,
					EventTypeUnknown,
				),
			}

			if !ctx.MatchesEventName(eventName) {
				t.Errorf(
					"rule event %q never matches: parsed context reports %v",
					eventName,
					ctx.EventNames(),
				)
			}
		})
	}
}

// Every event the bridge forwards must be matchable too, otherwise a rule
// author cannot scope a policy to something the plugin actually sends.
func TestOpenCodeForwardedEventsAreMatchable(t *testing.T) {
	for _, eventName := range OpenCodeEventNames() {
		t.Run(eventName, func(t *testing.T) {
			canonical := NormalizeEventName(eventName)
			if canonical == CanonicalEventUnknown {
				t.Fatalf("forwarded event %q does not normalize", eventName)
			}

			ctx := &Context{
				Provider: ProviderOpenCode,
				Event:    canonical,
				RawEventName: DisplayEventName(
					ProviderOpenCode,
					canonical,
					EventTypeUnknown,
				),
			}

			if !ctx.MatchesEventName(eventName) {
				t.Errorf(
					"forwarded event %q is not matchable: context reports %v",
					eventName,
					ctx.EventNames(),
				)
			}
		})
	}
}

// permission.ask is accepted from hand-written plugins but must not be
// advertised, since forwarding it alongside tool.execute.before would validate
// one call twice.
func TestPermissionAskIsNotForwarded(t *testing.T) {
	for _, eventName := range OpenCodeEventNames() {
		if eventName == "permission.ask" {
			t.Fatal("permission.ask must not be forwarded by the bridge plugin")
		}
	}

	if got := NormalizeEventName("permission.ask"); got != CanonicalEventBeforeTool {
		t.Errorf("NormalizeEventName(permission.ask) = %q, want %q", got, CanonicalEventBeforeTool)
	}
}
