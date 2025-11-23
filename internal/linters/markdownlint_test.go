package linters_test

import (
	"context"
	"errors"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
	"github.com/smykla-labs/claude-hooks/internal/linters"
)

var errMarkdownLintFailed = errors.New("markdownlint failed")

var _ = Describe("MarkdownLinter", func() {
	var (
		linter     linters.MarkdownLinter
		mockRunner *mockCommandRunner
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockRunner = &mockCommandRunner{}
		linter = linters.NewMarkdownLinter(mockRunner)
	})

	Describe("Lint", func() {
		Context("when markdownlint is not available", func() {
			It("should still run custom rules", func() {
				// Content with custom rule violation (no empty line before code block)
				content := `# Test
Some text
` + "```" + `
code
` + "```"

				// No CLI tool available, so runner won't be called
				result := linter.Lint(ctx, content)

				// Should fail due to custom rules
				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeFalse())
				Expect(result.RawOut).To(ContainSubstring("Custom rules:"))
				Expect(
					result.RawOut,
				).To(ContainSubstring("Code block should have empty line before it"))
			})
		})

		Context("when markdownlint succeeds with no issues and no custom rule violations", func() {
			It("should return success", func() {
				content := `# Test

Some text

` + "```" + `
code
` + "```"

				mockRunner.runWithStdinFunc = func(
					ctx context.Context,
					stdin io.Reader,
					name string,
					args ...string,
				) execpkg.CommandResult {
					Expect(name).To(Equal("markdownlint"))
					Expect(args).To(ContainElement("--stdin"))

					return execpkg.CommandResult{
						Stdout:   "",
						Stderr:   "",
						ExitCode: 0,
					}
				}

				result := linter.Lint(ctx, content)

				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeTrue())
				Expect(result.Err).To(BeNil())
			})
		})

		Context("when markdownlint finds issues", func() {
			It("should return failure with CLI output", func() {
				markdownlintOutput := `stdin:1 MD041/first-line-heading/first-line-h1 First line in file should be a top-level heading
stdin:5 MD022/blanks-around-headings/blanks-around-headers Headings should be surrounded by blank lines`

				mockRunner.runWithStdinFunc = func(
					ctx context.Context,
					stdin io.Reader,
					name string,
					args ...string,
				) execpkg.CommandResult {
					return execpkg.CommandResult{
						Stdout:   markdownlintOutput,
						Stderr:   "",
						ExitCode: 1,
						Err:      errMarkdownLintFailed,
					}
				}

				result := linter.Lint(ctx, "# Test\nSome content")

				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeFalse())
				Expect(result.RawOut).To(ContainSubstring("MD041"))
				Expect(result.Err).To(Equal(errMarkdownLintFailed))
			})
		})

		Context("when custom rules find issues", func() {
			It("should return failure with custom rule output", func() {
				// Content with custom rule violation (no empty line before code block)
				content := `# Test
Some text
` + "```" + `
code
` + "```"

				// markdownlint passes
				mockRunner.runWithStdinFunc = func(
					ctx context.Context,
					stdin io.Reader,
					name string,
					args ...string,
				) execpkg.CommandResult {
					return execpkg.CommandResult{
						Stdout:   "",
						Stderr:   "",
						ExitCode: 0,
					}
				}

				result := linter.Lint(ctx, content)

				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeFalse())
				Expect(result.RawOut).To(ContainSubstring("Custom rules:"))
				Expect(
					result.RawOut,
				).To(ContainSubstring("Code block should have empty line before it"))
				Expect(result.Err).To(Equal(linters.ErrMarkdownCustomRules))
			})
		})

		Context("when both markdownlint and custom rules find issues", func() {
			It("should combine both outputs", func() {
				content := `# Test
Some text
` + "```" + `
code
` + "```"

				markdownlintOutput := "stdin:1 MD041 First line should be heading"

				mockRunner.runWithStdinFunc = func(
					ctx context.Context,
					stdin io.Reader,
					name string,
					args ...string,
				) execpkg.CommandResult {
					return execpkg.CommandResult{
						Stdout:   markdownlintOutput,
						Stderr:   "",
						ExitCode: 1,
						Err:      errMarkdownLintFailed,
					}
				}

				result := linter.Lint(ctx, content)

				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeFalse())
				Expect(result.RawOut).To(ContainSubstring("MD041"))
				Expect(result.RawOut).To(ContainSubstring("Custom rules:"))
				Expect(result.Err).To(Equal(errMarkdownLintFailed))
			})
		})
	})
})
