package hookresponse_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/dispatcher"
	"github.com/smykla-skalski/klaudiush/internal/hookresponse"
	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

var _ = Describe("BuildOpenCode", func() {
	blockingErrs := func() []*dispatcher.ValidationError {
		return []*dispatcher.ValidationError{
			{
				Validator:   "git.push",
				Message:     "Force push is blocked",
				ShouldBlock: true,
				Reference:   validator.RefGitNoSignoff,
			},
		}
	}

	warningErrs := func() []*dispatcher.ValidationError {
		return []*dispatcher.ValidationError{
			{
				Validator:   "file.markdown",
				Message:     "Heading style",
				ShouldBlock: false,
			},
		}
	}

	ctxFor := func(event hook.CanonicalEvent) *hook.Context {
		return &hook.Context{
			Provider: hook.ProviderOpenCode,
			Event:    event,
			RawEventName: hook.DisplayEventName(
				hook.ProviderOpenCode,
				event,
				hook.EventTypeUnknown,
			),
		}
	}

	It("returns nil when there are no errors", func() {
		Expect(hookresponse.BuildOpenCode(ctxFor(hook.CanonicalEventBeforeTool), nil, nil)).
			To(BeNil())
	})

	It("denies a blocking pre-execution error", func() {
		resp := hookresponse.BuildOpenCode(
			ctxFor(hook.CanonicalEventBeforeTool),
			blockingErrs(),
			nil,
		)

		Expect(resp).NotTo(BeNil())
		Expect(resp.Decision).To(Equal("deny"))
		Expect(resp.Reason).To(ContainSubstring("Force push is blocked"))
		Expect(resp.SystemMessage).NotTo(BeEmpty())
	})

	It("blocks rather than denies on turn stop", func() {
		resp := hookresponse.BuildOpenCode(
			ctxFor(hook.CanonicalEventTurnStop),
			blockingErrs(),
			nil,
		)

		Expect(resp).NotTo(BeNil())
		Expect(resp.Decision).To(Equal("block"))
		Expect(resp.Reason).To(ContainSubstring("Force push is blocked"))
	})

	// The tool already ran, so findings are advisory context, never a denial.
	It("never denies after the tool ran", func() {
		resp := hookresponse.BuildOpenCode(
			ctxFor(hook.CanonicalEventAfterTool),
			blockingErrs(),
			nil,
		)

		Expect(resp).NotTo(BeNil())
		Expect(resp.Decision).To(BeEmpty())
		Expect(resp.HookSpecificOutput).NotTo(BeNil())
		Expect(resp.HookSpecificOutput.AdditionalContext).NotTo(BeEmpty())
		Expect(resp.HookSpecificOutput.HookEventName).To(Equal("tool.execute.after"))
	})

	It("passes warnings through as context without a decision", func() {
		resp := hookresponse.BuildOpenCode(
			ctxFor(hook.CanonicalEventBeforeTool),
			warningErrs(),
			nil,
		)

		Expect(resp).NotTo(BeNil())
		Expect(resp.Decision).To(BeEmpty())
		Expect(resp.Continue).To(BeTrue())
		Expect(resp.HookSpecificOutput).NotTo(BeNil())
		Expect(resp.HookSpecificOutput.AdditionalContext).NotTo(BeEmpty())
	})

	It("keeps lifecycle events advisory", func() {
		for _, event := range []hook.CanonicalEvent{
			hook.CanonicalEventSessionStart,
			hook.CanonicalEventNotification,
			hook.CanonicalEventPreCompress,
			hook.CanonicalEventPostCompact,
		} {
			resp := hookresponse.BuildOpenCode(ctxFor(event), blockingErrs(), nil)
			Expect(resp).NotTo(BeNil())
			Expect(resp.Decision).To(BeEmpty(), "event %s should not carry a decision", event)
		}
	})

	It("denies a blocking user prompt", func() {
		resp := hookresponse.BuildOpenCode(
			ctxFor(hook.CanonicalEventUserPromptSubmit),
			blockingErrs(),
			nil,
		)

		Expect(resp).NotTo(BeNil())
		Expect(resp.Decision).To(Equal("deny"))
	})

	It("is selected by BuildForContext for opencode contexts", func() {
		resp := hookresponse.BuildForContext(
			ctxFor(hook.CanonicalEventBeforeTool),
			blockingErrs(),
			nil,
		)

		typed, ok := resp.(*hookresponse.OpenCodeCommandResponse)
		Expect(ok).To(BeTrue())
		Expect(typed.Decision).To(Equal("deny"))
	})
})

var _ = Describe("opencode notices", func() {
	openCodeCtx := &hook.Context{
		Provider: hook.ProviderOpenCode,
		Event:    hook.CanonicalEventBeforeTool,
	}

	It("builds an opencode-shaped notice", func() {
		notice := hookresponse.BuildNotice(openCodeCtx, "update available")

		typed, ok := notice.(*hookresponse.OpenCodeCommandResponse)
		Expect(ok).To(BeTrue())
		Expect(typed.SystemMessage).To(Equal("update available"))
		Expect(typed.Continue).To(BeTrue())
		Expect(hookresponse.IsEmpty(notice)).To(BeFalse())
	})

	It("appends notices to an existing opencode response", func() {
		resp := &hookresponse.OpenCodeCommandResponse{
			Continue:      true,
			SystemMessage: "first",
		}

		hookresponse.AppendNotice(resp, "second")
		Expect(resp.SystemMessage).To(Equal("first\n\nsecond"))
	})

	It("treats a nil opencode response as empty", func() {
		var resp *hookresponse.OpenCodeCommandResponse

		Expect(hookresponse.IsEmpty(resp)).To(BeTrue())
	})
})
