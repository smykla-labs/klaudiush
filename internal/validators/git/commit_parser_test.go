package git

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseTitle", func() {
	Describe("valid titles", func() {
		It("parses valid title with scope", func() {
			result := parseTitle("feat(api): add endpoint")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("feat"))
			Expect(result.Scope).To(Equal("api"))
			Expect(result.Description).To(Equal("add endpoint"))
			Expect(result.Exclamation).To(BeFalse())
		})

		It("parses breaking change marker", func() {
			result := parseTitle("feat(api)!: breaking change")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("feat"))
			Expect(result.Scope).To(Equal("api"))
			Expect(result.Exclamation).To(BeTrue())
			Expect(result.Description).To(Equal("breaking change"))
		})

		It("parses title without scope", func() {
			result := parseTitle("fix: bug fix")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("fix"))
			Expect(result.Scope).To(BeEmpty())
			Expect(result.Description).To(Equal("bug fix"))
			Expect(result.Exclamation).To(BeFalse())
		})

		It("parses title without scope with breaking change", func() {
			result := parseTitle("fix!: breaking bug fix")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("fix"))
			Expect(result.Scope).To(BeEmpty())
			Expect(result.Exclamation).To(BeTrue())
			Expect(result.Description).To(Equal("breaking bug fix"))
		})

		It("parses title with complex scope", func() {
			result := parseTitle("feat(api/v2): add new version")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("feat"))
			Expect(result.Scope).To(Equal("api/v2"))
			Expect(result.Description).To(Equal("add new version"))
		})

		It("parses title with hyphenated scope", func() {
			result := parseTitle("ci(github-actions): add workflow")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("ci"))
			Expect(result.Scope).To(Equal("github-actions"))
			Expect(result.Description).To(Equal("add workflow"))
		})

		It("parses title with underscore scope", func() {
			result := parseTitle("test(unit_tests): add coverage")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Scope).To(Equal("unit_tests"))
		})

		It("parses all standard commit types", func() {
			types := []string{
				"feat", "fix", "docs", "style", "refactor",
				"perf", "test", "build", "ci", "chore", "revert",
			}

			for _, t := range types {
				result := parseTitle(t + "(scope): description")

				Expect(result.Valid).To(BeTrue(), "type %s should be valid", t)
				Expect(result.Type).To(Equal(t))
			}
		})
	})

	Describe("invalid titles", func() {
		It("returns invalid for non-conventional title", func() {
			result := parseTitle("Add new feature")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for missing colon", func() {
			result := parseTitle("feat(api) add endpoint")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for missing description", func() {
			result := parseTitle("feat(api):")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for missing space after colon", func() {
			result := parseTitle("feat(api):no space")

			Expect(result.Valid).To(BeFalse())
		})

		It("handles empty string", func() {
			result := parseTitle("")

			Expect(result.Valid).To(BeFalse())
		})

		It("handles whitespace only", func() {
			result := parseTitle("   ")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for empty scope", func() {
			result := parseTitle("feat(): add endpoint")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for special characters in scope", func() {
			result := parseTitle("feat(@api): add endpoint")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for scope starting with slash", func() {
			result := parseTitle("feat(/api): add endpoint")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for scope ending with slash", func() {
			result := parseTitle("feat(api/): add endpoint")

			Expect(result.Valid).To(BeFalse())
		})

		It("returns invalid for scope with only special character", func() {
			result := parseTitle("feat(-): add endpoint")

			Expect(result.Valid).To(BeFalse())
		})
	})

	Describe("edge cases", func() {
		It("handles description with special characters", func() {
			result := parseTitle("fix(auth): resolve issue #123 with OAuth2.0")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Description).To(Equal("resolve issue #123 with OAuth2.0"))
		})

		It("handles description with emojis", func() {
			result := parseTitle("feat(ui): add dark mode toggle ✨")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Description).To(Equal("add dark mode toggle ✨"))
		})

		It("handles multi-word type (custom types)", func() {
			result := parseTitle("customtype(scope): description")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("customtype"))
		})

		It("handles numeric scope", func() {
			result := parseTitle("fix(123): bug in module 123")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Scope).To(Equal("123"))
		})

		It("preserves description whitespace", func() {
			result := parseTitle("feat(api): add  multiple  spaces")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Description).To(Equal("add  multiple  spaces"))
		})

		It("handles description ending with punctuation", func() {
			result := parseTitle("fix(api): resolve bug.")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Description).To(Equal("resolve bug."))
		})
	})
})

var _ = Describe("CommitParser", func() {
	var parser *CommitParser

	BeforeEach(func() {
		parser = NewCommitParser()
	})

	Describe("Parse", func() {
		It("parses empty message", func() {
			result := parser.Parse("")

			Expect(result.Valid).To(BeFalse())
			Expect(result.Raw).To(BeEmpty())
		})

		It("parses simple conventional commit", func() {
			result := parser.Parse("feat(api): add endpoint")

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("feat"))
			Expect(result.Scope).To(Equal("api"))
			Expect(result.Description).To(Equal("add endpoint"))
			Expect(result.Title).To(Equal("feat(api): add endpoint"))
		})

		It("parses commit with body", func() {
			message := `feat(api): add endpoint

This adds a new API endpoint for user management.`

			result := parser.Parse(message)

			Expect(result.Valid).To(BeTrue())
			Expect(result.Body).To(Equal("This adds a new API endpoint for user management."))
		})

		It("parses commit with footers", func() {
			message := `feat(api): add endpoint

Add new endpoint.

Refs: #123
Reviewed-by: Jane`

			result := parser.Parse(message)

			Expect(result.Valid).To(BeTrue())
			Expect(result.Footers).To(HaveKey("Refs"))
			Expect(result.Footers).To(HaveKey("Reviewed-by"))
			Expect(result.Footers["Refs"]).To(ContainElement("#123"))
		})

		It("detects BREAKING CHANGE in title", func() {
			result := parser.Parse("feat(api)!: remove old endpoint")

			Expect(result.Valid).To(BeTrue())
			Expect(result.IsBreakingChange).To(BeTrue())
		})

		It("detects BREAKING CHANGE in footer", func() {
			message := `feat(api): update endpoint

Change response format.

BREAKING CHANGE: response format has changed`

			result := parser.Parse(message)

			Expect(result.Valid).To(BeTrue())
			Expect(result.IsBreakingChange).To(BeTrue())
		})

		It("detects BREAKING-CHANGE in footer", func() {
			message := `feat(api): update endpoint

BREAKING-CHANGE: response format has changed`

			result := parser.Parse(message)

			Expect(result.Valid).To(BeTrue())
			Expect(result.IsBreakingChange).To(BeTrue())
		})

		It("handles revert commit", func() {
			result := parser.Parse(`Revert "feat(api): add endpoint"`)

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("revert"))
		})

		It("handles revert commit with single quotes", func() {
			result := parser.Parse(`Revert 'feat(api): add endpoint'`)

			Expect(result.Valid).To(BeTrue())
			Expect(result.Type).To(Equal("revert"))
		})

		It("validates commit type against valid types", func() {
			result := parser.Parse("invalid(api): add endpoint")

			Expect(result.Valid).To(BeFalse())
			Expect(result.ParseError).To(ContainSubstring("invalid commit type"))
		})

		It("returns error for non-conventional format", func() {
			result := parser.Parse("Add new feature")

			Expect(result.Valid).To(BeFalse())
			Expect(result.ParseError).To(ContainSubstring("failed to parse as conventional commit"))
		})

		It("handles commit with trailer-like patterns in body", func() {
			message := `build(makefile): fix version script

Solution: Use foreach to evaluate each line separately,
ensuring each variable assignment is processed independently.`

			result := parser.Parse(message)

			Expect(result.Valid).To(BeTrue())
			Expect(result.Body).To(ContainSubstring("Solution:"))
		})
	})

	Describe("WithValidTypes", func() {
		It("accepts custom valid types", func() {
			customParser := NewCommitParser(WithValidTypes([]string{"custom", "special"}))

			result := customParser.Parse("custom(api): add feature")

			Expect(result.Valid).To(BeTrue())
		})

		It("rejects types not in custom list", func() {
			customParser := NewCommitParser(WithValidTypes([]string{"custom", "special"}))

			result := customParser.Parse("feat(api): add feature")

			Expect(result.Valid).To(BeFalse())
			Expect(result.ParseError).To(ContainSubstring("invalid commit type"))
		})
	})
})
