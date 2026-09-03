package parser_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/parser"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

var _ = Describe("JSONParser opencode payloads", func() {
	parse := func(input, eventName string) *hook.Context {
		p := parser.NewJSONParser(bytes.NewReader([]byte(input)))

		ctx, err := p.ParseWithOptions(parser.ParseOptions{
			Provider:  hook.ProviderOpenCode,
			EventType: hook.EventTypeUnknown,
			EventName: eventName,
		})
		Expect(err).NotTo(HaveOccurred())

		return ctx
	}

	It("parses a bash tool call from the bridge plugin", func() {
		input := `{
			"hook_event_name": "tool.execute.before",
			"session_id": "ses_123",
			"cwd": "/repo",
			"tool_name": "bash",
			"tool_input": {"command": "git push --force"},
			"tool_use_id": "call_1"
		}`

		ctx := parse(input, "tool.execute.before")

		Expect(ctx.Provider).To(Equal(hook.ProviderOpenCode))
		Expect(ctx.Event).To(Equal(hook.CanonicalEventBeforeTool))
		Expect(ctx.EventType).To(Equal(hook.EventTypePreToolUse))
		Expect(ctx.ToolName).To(Equal(hook.ToolTypeBash))
		Expect(ctx.ToolFamily).To(Equal(hook.ToolFamilyShell))
		Expect(ctx.GetCommand()).To(Equal("git push --force"))
		Expect(ctx.GetWorkingDir()).To(Equal("/repo"))
		Expect(ctx.SessionID).To(Equal("ses_123"))
		Expect(ctx.ToolUseID).To(Equal("call_1"))
	})

	// opencode names tool arguments in camelCase, unlike Claude's snake_case.
	It("reads camelCase tool arguments", func() {
		input := `{
			"hook_event_name": "tool.execute.before",
			"session_id": "ses_123",
			"cwd": "/repo",
			"tool_name": "edit",
			"tool_input": {
				"filePath": "/repo/main.go",
				"oldString": "before",
				"newString": "after"
			}
		}`

		ctx := parse(input, "tool.execute.before")

		Expect(ctx.ToolName).To(Equal(hook.ToolTypeEdit))
		Expect(ctx.ToolInput.FilePath).To(Equal("/repo/main.go"))
		Expect(ctx.ToolInput.OldString).To(Equal("before"))
		Expect(ctx.ToolInput.NewString).To(Equal("after"))
		Expect(ctx.AffectedPaths).To(ContainElement("/repo/main.go"))
	})

	It("treats permission.ask as a decisive pre-execution gate", func() {
		input := `{
			"hook_event_name": "permission.ask",
			"session_id": "ses_123",
			"cwd": "/repo",
			"tool_name": "bash",
			"tool_input": {},
			"command": "git commit -m nope"
		}`

		ctx := parse(input, "permission.ask")

		Expect(ctx.Event).To(Equal(hook.CanonicalEventBeforeTool))
		Expect(ctx.EventType).To(Equal(hook.EventTypePreToolUse))
		// The top-level command is the fallback when metadata carries no args.
		Expect(ctx.GetCommand()).To(Equal("git commit -m nope"))
	})

	It("maps lifecycle events onto canonical events", func() {
		cases := map[string]hook.CanonicalEvent{
			"session.created":    hook.CanonicalEventSessionStart,
			"session.idle":       hook.CanonicalEventTurnStop,
			"session.compacted":  hook.CanonicalEventPostCompact,
			"session.compacting": hook.CanonicalEventPreCompress,
			"permission.updated": hook.CanonicalEventNotification,
			"chat.message":       hook.CanonicalEventUserPromptSubmit,
			"tool.execute.after": hook.CanonicalEventAfterTool,
		}

		for eventName, expected := range cases {
			input := `{"hook_event_name": "` + eventName + `", "session_id": "s", "cwd": "/repo"}`

			ctx := parse(input, eventName)
			Expect(ctx.Event).To(Equal(expected), "event %s", eventName)
		}
	})

	It("infers the opencode provider from a dotted event name", func() {
		input := `{
			"hook_event_name": "tool.execute.before",
			"tool_name": "bash",
			"tool_input": {"command": "ls"}
		}`

		p := parser.NewJSONParser(bytes.NewReader([]byte(input)))

		ctx, err := p.ParseWithOptions(parser.ParseOptions{
			Provider:  hook.ProviderUnknown,
			EventType: hook.EventTypeUnknown,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(ctx.Provider).To(Equal(hook.ProviderOpenCode))
		Expect(ctx.Event).To(Equal(hook.CanonicalEventBeforeTool))
	})

	It("still infers Claude for undotted payloads", func() {
		input := `{
			"hook_event_name": "PreToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "ls"}
		}`

		p := parser.NewJSONParser(bytes.NewReader([]byte(input)))

		ctx, err := p.ParseWithOptions(parser.ParseOptions{
			Provider:  hook.ProviderUnknown,
			EventType: hook.EventTypeUnknown,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(ctx.Provider).To(Equal(hook.ProviderClaude))
	})

	It("echoes the opencode hook id back as the raw event name", func() {
		input := `{"hook_event_name": "tool.execute.after", "session_id": "s", "cwd": "/repo"}`

		ctx := parse(input, "tool.execute.after")
		Expect(ctx.RawEventName).To(Equal("tool.execute.after"))
	})
})
