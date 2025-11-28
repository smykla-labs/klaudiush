package github_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/smykla-labs/klaudiush/internal/linters"
	"github.com/smykla-labs/klaudiush/internal/validators/github"
	"github.com/smykla-labs/klaudiush/pkg/config"
	"github.com/smykla-labs/klaudiush/pkg/hook"
	"github.com/smykla-labs/klaudiush/pkg/logger"
)

var _ = Describe("IssueValidator", func() {
	var (
		validator  *github.IssueValidator
		mockCtrl   *gomock.Controller
		mockLinter *linters.MockMarkdownLinter
		ctx        context.Context
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockLinter = linters.NewMockMarkdownLinter(mockCtrl)
		validator = github.NewIssueValidator(nil, mockLinter, logger.NewNoOpLogger(), nil)
		ctx = context.Background()
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("Validate", func() {
		It("should pass for command without body", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug report"`,
				},
			}

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should pass for non-gh commands", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `git commit -m "test"`,
				},
			}

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should validate body with --body flag", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug report" --body "### Description

This is a bug description.

### Steps to Reproduce

1. Step one
2. Step two
"`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{Success: true})

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should validate body with heredoc", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug report" --body "$(cat <<'EOF'
### Description

This is a bug description.

### Steps to Reproduce

1. Step one
2. Step two
EOF
)"`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{Success: true})

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should warn for markdown errors", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug report" --body "### Description
No empty line after heading"`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{
					Success: false,
					RawOut:  "MD022 Headings should be surrounded by blank lines",
				})

			result := validator.Validate(ctx, hookCtx)
			// By default, issue validator only warns
			Expect(result.ShouldBlock).To(BeFalse())
			Expect(result.Message).To(ContainSubstring("markdown validation"))
		})
	})

	Describe("with RequireBody enabled", func() {
		BeforeEach(func() {
			requireBody := true
			cfg := &config.IssueValidatorConfig{
				RequireBody: &requireBody,
			}
			validator = github.NewIssueValidator(cfg, mockLinter, logger.NewNoOpLogger(), nil)
		})

		It("should fail when body is missing", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug report"`,
				},
			}

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeFalse())
			Expect(result.ShouldBlock).To(BeTrue())
			Expect(result.Message).To(ContainSubstring("Issue body is required"))
		})
	})

	Describe("extractIssueData", func() {
		It("should extract title from double quotes", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "My bug report"`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{Success: true}).
				AnyTimes()

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should extract title from single quotes", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title 'My bug report'`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{Success: true}).
				AnyTimes()

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should extract body from double quotes", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug" --body "Body content here"`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{Success: true})

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should extract body from single quotes", func() {
			hookCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeBash,
				ToolInput: hook.ToolInput{
					Command: `gh issue create --title "Bug" --body 'Body content here'`,
				},
			}

			mockLinter.EXPECT().
				Lint(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&linters.LintResult{Success: true})

			result := validator.Validate(ctx, hookCtx)
			Expect(result.Passed).To(BeTrue())
		})
	})

	Describe("filterDisabledRules", func() {
		It("should filter out disabled rules from output", func() {
			output := `file.md:1 MD013 Line too long
file.md:3 MD022 Headings should be surrounded by blank lines
file.md:5 MD041 First line in a file should be a top-level heading`

			filtered := github.FilterDisabledRules(output, []string{"MD013", "MD041"})

			Expect(filtered).To(ContainSubstring("MD022"))
			Expect(filtered).NotTo(ContainSubstring("MD013"))
			Expect(filtered).NotTo(ContainSubstring("MD041"))
		})

		It("should return original output when no rules disabled", func() {
			output := "file.md:1 MD022 Headings should be surrounded by blank lines"

			filtered := github.FilterDisabledRules(output, []string{})

			Expect(filtered).To(Equal(output))
		})
	})

	Describe("Category", func() {
		It("should return CategoryIO", func() {
			Expect(validator.Category()).To(Equal(github.CategoryIO))
		})
	})
})
