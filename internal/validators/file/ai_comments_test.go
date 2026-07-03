package file_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/internal/validators/file"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

var _ = Describe("AICommentValidator", func() {
	var (
		v   *file.AICommentValidator
		ctx *hook.Context
	)

	BeforeEach(func() {
		v = file.NewAICommentValidator(logger.NewNoOpLogger(), nil, nil)
		ctx = &hook.Context{
			EventType: hook.EventTypePreToolUse,
			ToolName:  hook.ToolTypeWrite,
		}
	})

	Describe("Name", func() {
		It("returns correct validator name", func() {
			Expect(v.Name()).To(Equal("validate-ai-comments"))
		})
	})

	Describe("Category", func() {
		It("returns CategoryCPU", func() {
			Expect(v.Category()).To(Equal(validator.CategoryCPU))
		})
	})

	Describe("NewAICommentValidator", func() {
		It("handles invalid regex patterns gracefully", func() {
			cfg := &config.AICommentValidatorConfig{
				Patterns: []string{
					`valid-pattern`,
					`[invalid(regex`, // Invalid regex
					`another-valid`,
				},
			}

			validator := file.NewAICommentValidator(logger.NewNoOpLogger(), cfg, nil)
			Expect(validator).NotTo(BeNil())
		})

		It("uses default patterns when config is nil", func() {
			validator := file.NewAICommentValidator(logger.NewNoOpLogger(), nil, nil)
			Expect(validator).NotTo(BeNil())
		})

		It("uses custom patterns from config", func() {
			cfg := &config.AICommentValidatorConfig{
				Patterns: []string{`// custom-filler`},
			}

			validator := file.NewAICommentValidator(logger.NewNoOpLogger(), cfg, nil)
			testCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeWrite,
				ToolInput: hook.ToolInput{
					Content: `x := 1 // custom-filler`,
				},
			}

			result := validator.Validate(context.Background(), testCtx)
			Expect(result.Passed).To(BeFalse())
		})

		It("uses default patterns when config has empty patterns", func() {
			cfg := &config.AICommentValidatorConfig{
				Patterns: []string{},
			}

			validator := file.NewAICommentValidator(logger.NewNoOpLogger(), cfg, nil)
			testCtx := &hook.Context{
				EventType: hook.EventTypePreToolUse,
				ToolName:  hook.ToolTypeWrite,
				ToolInput: hook.ToolInput{
					Content: `// Initialize the counter
count := 0`,
				},
			}

			result := validator.Validate(context.Background(), testCtx)
			Expect(result.Passed).To(BeFalse())
		})
	})

	Describe("Validate", func() {
		Context("with clean code", func() {
			It("passes for code with no filler comments", func() {
				ctx.ToolInput.Content = `
// ExecuteQuery runs against a read replica to avoid write-path lock contention.
func ExecuteQuery() {}
`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeTrue())
			})

			It("passes for empty content", func() {
				ctx.ToolInput.Content = ""
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeTrue())
			})
		})

		Context("filler comment phrases", func() {
			It("fails for 'Initialize the'", func() {
				ctx.ToolInput.Content = `
// Initialize the counter
count := 0
`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
				Expect(result.ShouldBlock).To(BeTrue())
				Expect(
					result.Message,
				).To(ContainSubstring("Filler comments that only restate the code are not allowed"))
			})

			It("fails for 'Loop through'", func() {
				ctx.ToolInput.Content = `// Loop through the items
for _, item := range items {}`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
			})

			It("fails for 'Check if'", func() {
				ctx.ToolInput.Content = `# Check if the value is set
if value:
    pass`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
			})

			It("fails for 'Return the'", func() {
				ctx.ToolInput.Content = `// Return the result
return result`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
			})

			It("fails for 'Create a new'", func() {
				ctx.ToolInput.Content = "// Create a new session\nsess := Session{}"
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
			})

			It("fails for 'This function'", func() {
				ctx.ToolInput.Content = `// This function does the calculation
func calc() {}`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
			})
		})

		Context("Edit operations", func() {
			BeforeEach(func() {
				ctx.ToolName = hook.ToolTypeEdit
			})

			It("checks new_string content", func() {
				ctx.ToolInput.OldString = `count := 0`
				ctx.ToolInput.NewString = "// Initialize the counter\ncount := 0"
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
			})

			It("passes when new_string is clean", func() {
				ctx.ToolInput.OldString = "// Initialize the counter\ncount := 0"
				ctx.ToolInput.NewString = `count := 0`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeTrue())
			})
		})

		Context("error reference", func() {
			It("includes FILE011 reference", func() {
				ctx.ToolInput.Content = `// Initialize the counter`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
				Expect(string(result.Reference)).To(Equal("https://klaudiu.sh/e/FILE011"))
			})

			It("includes fix hint", func() {
				ctx.ToolInput.Content = `// Initialize the counter`
				result := v.Validate(context.Background(), ctx)
				Expect(result.Passed).To(BeFalse())
				Expect(result.FixHint).To(ContainSubstring("explains why, not what"))
			})
		})
	})
})
