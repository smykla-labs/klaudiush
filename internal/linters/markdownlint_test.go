package linters_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-labs/klaudiush/internal/linters"
)

var _ = Describe("MarkdownLinter", func() {
	var (
		linter linters.MarkdownLinter
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		linter = linters.NewMarkdownLinter(nil) // runner not used for custom rules only
	})

	Describe("Lint", func() {
		Context("when content has custom rule violations", func() {
			It("should fail with custom rule output", func() {
				// Content with custom rule violation (no empty line before code block)
				content := `# Test
Some text
` + "```" + `
code
` + "```"

				result := linter.Lint(ctx, content, nil)

				// Should fail due to custom rules
				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeFalse())
				Expect(
					result.RawOut,
				).To(ContainSubstring("Code block should have empty line before it"))
			})
		})

		Context("when content has no custom rule violations", func() {
			It("should return success", func() {
				content := `# Test

Some text

` + "```" + `
code
` + "```"

				result := linter.Lint(ctx, content, nil)

				Expect(result).NotTo(BeNil())
				Expect(result.Success).To(BeTrue())
				Expect(result.Err).To(BeNil())
			})
		})
	})

	Describe("Tool Detection Functions", func() {
		DescribeTable("isMarkdownlintCli2",
			func(toolPath string, expected bool) {
				result := linters.IsMarkdownlintCli2(toolPath)
				Expect(result).To(Equal(expected))
			},
			Entry("markdownlint-cli2 binary", "/usr/local/bin/markdownlint-cli2", true),
			Entry("markdownlint-cli binary", "/usr/local/bin/markdownlint-cli", false),
			Entry("markdownlint binary", "/usr/local/bin/markdownlint", false),
			Entry("custom markdownlint-cli2 wrapper path",
				"/usr/bin/my-markdownlint-cli2-wrapper", true),
			Entry("custom markdownlint wrapper path",
				"/usr/bin/my-markdownlint-wrapper", false),
			Entry("markdownlint-cli2 in node_modules",
				"/path/to/node_modules/.bin/markdownlint-cli2", true),
			Entry("markdownlint-cli in node_modules",
				"/path/to/node_modules/.bin/markdownlint-cli", false),
			Entry("empty path", "", false),
		)

		DescribeTable("GenerateFragmentConfigContent",
			func(isCli2, disableMD047 bool, expectedContent string) {
				result := linters.GenerateFragmentConfigContent(isCli2, disableMD047)
				Expect(result).To(Equal(expectedContent))
			},
			Entry("markdownlint-cli2 with MD047 disabled", true, true, `{
  "config": {
    "MD047": false
  }
}`),
			Entry("markdownlint-cli2 with MD047 enabled", true, false, `{
  "config": {}
}`),
			Entry("markdownlint-cli with MD047 disabled", false, true, `{
  "MD047": false
}`),
			Entry("markdownlint-cli with MD047 enabled", false, false, "{}"),
		)

		DescribeTable("GetFragmentConfigPattern",
			func(isCli2 bool, expectedPattern string) {
				result := linters.GetFragmentConfigPattern(isCli2)
				Expect(result).To(Equal(expectedPattern))
			},
			Entry("markdownlint-cli2 pattern",
				true, "fragment-*.markdownlint-cli2.jsonc"),
			Entry("markdownlint-cli pattern",
				false, "markdownlint-fragment-*.json"),
		)
	})
})
