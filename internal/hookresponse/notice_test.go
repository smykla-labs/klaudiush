package hookresponse_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/hookresponse"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

var _ = Describe("Notice", func() {
	msg := "Update available: klaudiush 1.0.0 -> 2.0.0. Run 'klaudiush update' to install."

	Describe("BuildNotice", func() {
		It("builds HookResponse for Claude provider", func() {
			ctx := &hook.Context{Provider: hook.ProviderClaude}
			resp := hookresponse.BuildNotice(ctx, msg)

			hr, ok := resp.(*hookresponse.HookResponse)
			Expect(ok).To(BeTrue())
			Expect(hr.SystemMessage).To(Equal(msg))
			Expect(hr.HookSpecificOutput).To(BeNil())
		})

		It("builds CodexCommandResponse for Codex provider", func() {
			ctx := &hook.Context{Provider: hook.ProviderCodex}
			resp := hookresponse.BuildNotice(ctx, msg)

			cr, ok := resp.(*hookresponse.CodexCommandResponse)
			Expect(ok).To(BeTrue())
			Expect(cr.SystemMessage).To(Equal(msg))
			Expect(cr.Continue).To(BeTrue())
		})

		It("builds GeminiCommandResponse for Gemini provider", func() {
			ctx := &hook.Context{Provider: hook.ProviderGemini}
			resp := hookresponse.BuildNotice(ctx, msg)

			gr, ok := resp.(*hookresponse.GeminiCommandResponse)
			Expect(ok).To(BeTrue())
			Expect(gr.SystemMessage).To(Equal(msg))
		})

		It("builds HookResponse for nil context", func() {
			resp := hookresponse.BuildNotice(nil, msg)

			hr, ok := resp.(*hookresponse.HookResponse)
			Expect(ok).To(BeTrue())
			Expect(hr.SystemMessage).To(Equal(msg))
		})
	})

	Describe("AppendNotice", func() {
		It("appends to HookResponse", func() {
			resp := &hookresponse.HookResponse{SystemMessage: "existing error"}
			hookresponse.AppendNotice(resp, msg)
			Expect(resp.SystemMessage).To(Equal("existing error\n\n" + msg))
		})

		It("appends to CodexCommandResponse", func() {
			resp := &hookresponse.CodexCommandResponse{SystemMessage: "existing"}
			hookresponse.AppendNotice(resp, msg)
			Expect(resp.SystemMessage).To(Equal("existing\n\n" + msg))
		})

		It("appends to GeminiCommandResponse", func() {
			resp := &hookresponse.GeminiCommandResponse{SystemMessage: "existing"}
			hookresponse.AppendNotice(resp, msg)
			Expect(resp.SystemMessage).To(Equal("existing\n\n" + msg))
		})

		It("appends to ElicitationHookResponse", func() {
			resp := &hookresponse.ElicitationHookResponse{SystemMessage: "existing"}
			hookresponse.AppendNotice(resp, msg)
			Expect(resp.SystemMessage).To(Equal("existing\n\n" + msg))
		})

		It("skips the separator when there is no existing message", func() {
			resp := &hookresponse.HookResponse{}
			hookresponse.AppendNotice(resp, msg)
			Expect(resp.SystemMessage).To(Equal(msg))
		})
	})

	Describe("IsEmpty", func() {
		It("reports an untyped nil as empty", func() {
			Expect(hookresponse.IsEmpty(nil)).To(BeTrue())
		})

		It("reports typed nil pointers as empty", func() {
			var (
				claude      *hookresponse.HookResponse
				codex       *hookresponse.CodexCommandResponse
				gemini      *hookresponse.GeminiCommandResponse
				elicitation *hookresponse.ElicitationHookResponse
			)

			Expect(hookresponse.IsEmpty(claude)).To(BeTrue())
			Expect(hookresponse.IsEmpty(codex)).To(BeTrue())
			Expect(hookresponse.IsEmpty(gemini)).To(BeTrue())
			Expect(hookresponse.IsEmpty(elicitation)).To(BeTrue())
		})

		It("reports built responses as non-empty", func() {
			Expect(hookresponse.IsEmpty(&hookresponse.HookResponse{})).To(BeFalse())
		})
	})
})
