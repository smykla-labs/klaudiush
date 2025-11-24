package git_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-labs/klaudiush/internal/validators/git"
	"github.com/smykla-labs/klaudiush/pkg/hook"
	"github.com/smykla-labs/klaudiush/pkg/logger"
)

var _ = Describe("PRValidator", func() {
	var validator *git.PRValidator

	BeforeEach(func() {
		validator = git.NewPRValidator(logger.NewNoOpLogger())
	})

	Describe("Title Validation", func() {
		It("should pass for valid semantic commit title", func() {
			result := git.ValidatePRTitle("feat(api): add new endpoint")
			Expect(result.Valid).To(BeTrue())
		})

		It("should pass for title without scope", func() {
			result := git.ValidatePRTitle("feat: add new feature")
			Expect(result.Valid).To(BeTrue())
		})

		It("should pass for breaking change marker", func() {
			result := git.ValidatePRTitle("feat!: breaking API change")
			Expect(result.Valid).To(BeTrue())
		})

		It("should fail for invalid format", func() {
			result := git.ValidatePRTitle("Add new feature")
			Expect(result.Valid).To(BeFalse())
			Expect(
				result.ErrorMessage,
			).To(ContainSubstring("doesn't follow semantic commit format"))
		})

		It("should fail for empty title", func() {
			result := git.ValidatePRTitle("")
			Expect(result.Valid).To(BeFalse())
			Expect(result.ErrorMessage).To(Equal("PR title is empty"))
		})

		It("should fail for feat(ci)", func() {
			result := git.ValidatePRTitle("feat(ci): add new workflow")
			Expect(result.Valid).To(BeFalse())
			Expect(result.ErrorMessage).To(ContainSubstring("Use 'ci(...)' not 'feat(ci)'"))
		})

		It("should fail for fix(test)", func() {
			result := git.ValidatePRTitle("fix(test): update test")
			Expect(result.Valid).To(BeFalse())
			Expect(result.ErrorMessage).To(ContainSubstring("Use 'test(...)' not 'fix(test)'"))
		})

		It("should fail for feat(docs)", func() {
			result := git.ValidatePRTitle("feat(docs): add documentation")
			Expect(result.Valid).To(BeFalse())
			Expect(result.ErrorMessage).To(ContainSubstring("Use 'docs(...)' not 'feat(docs)'"))
		})

		It("should fail for fix(build)", func() {
			result := git.ValidatePRTitle("fix(build): update build")
			Expect(result.Valid).To(BeFalse())
			Expect(result.ErrorMessage).To(ContainSubstring("Use 'build(...)' not 'fix(build)'"))
		})
	})

	Describe("Type Extraction", func() {
		It("should extract feat type", func() {
			prType := git.ExtractPRType("feat(api): add endpoint")
			Expect(prType).To(Equal("feat"))
		})

		It("should extract ci type", func() {
			prType := git.ExtractPRType("ci(workflow): update pipeline")
			Expect(prType).To(Equal("ci"))
		})

		It("should return empty for invalid title", func() {
			prType := git.ExtractPRType("Invalid title")
			Expect(prType).To(Equal(""))
		})
	})

	Describe("Non-User-Facing Type Check", func() {
		It("should identify ci as non-user-facing", func() {
			Expect(git.IsNonUserFacingType("ci")).To(BeTrue())
		})

		It("should identify test as non-user-facing", func() {
			Expect(git.IsNonUserFacingType("test")).To(BeTrue())
		})

		It("should identify chore as non-user-facing", func() {
			Expect(git.IsNonUserFacingType("chore")).To(BeTrue())
		})

		It("should identify feat as user-facing", func() {
			Expect(git.IsNonUserFacingType("feat")).To(BeFalse())
		})

		It("should identify fix as user-facing", func() {
			Expect(git.IsNonUserFacingType("fix")).To(BeFalse())
		})
	})

	Describe("Body Validation", func() {
		It("should pass for valid body with all sections", func() {
			body := `## Motivation
This change improves performance.

## Implementation information
- Updated algorithm
- Added caching

## Supporting documentation
See docs/performance.md`

			result := git.ValidatePRBody(body, "feat")
			Expect(result.Errors).To(BeEmpty())
		})

		It("should error on missing Motivation section", func() {
			body := `## Implementation information
- Updated algorithm

## Supporting documentation
See docs/performance.md`

			result := git.ValidatePRBody(body, "feat")
			Expect(result.Errors).To(ContainElement(ContainSubstring("missing '## Motivation'")))
		})

		It("should error on missing Implementation information section", func() {
			body := `## Motivation
This change improves performance.

## Supporting documentation
See docs/performance.md`

			result := git.ValidatePRBody(body, "feat")
			Expect(
				result.Errors,
			).To(ContainElement(ContainSubstring("missing '## Implementation information'")))
		})

		It("should error on missing Supporting documentation section", func() {
			body := `## Motivation
This change improves performance.

## Implementation information
- Updated algorithm`

			result := git.ValidatePRBody(body, "feat")
			Expect(
				result.Errors,
			).To(ContainElement(ContainSubstring("missing '## Supporting documentation'")))
		})

		It("should warn on empty body", func() {
			result := git.ValidatePRBody("", "feat")
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("Could not extract PR body")))
		})

		It("should warn for ci type without changelog skip", func() {
			body := `## Motivation
CI change

## Implementation information
- Updated workflow

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "ci")
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("should typically have '> Changelog: skip'")))
		})

		It("should not warn for ci type with changelog skip", func() {
			body := `## Motivation
CI change

> Changelog: skip

## Implementation information
- Updated workflow

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "ci")
			Expect(
				result.Warnings,
			).NotTo(ContainElement(ContainSubstring("should typically have '> Changelog: skip'")))
		})

		It("should warn for feat type with changelog skip", func() {
			body := `## Motivation
New feature

> Changelog: skip

## Implementation information
- Added endpoint

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "feat")
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("is user-facing but has 'Changelog: skip'")))
		})

		It("should validate custom changelog format", func() {
			body := `## Motivation
New feature

> Changelog: invalid changelog format

## Implementation information
- Added endpoint

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "feat")
			Expect(
				result.Errors,
			).To(ContainElement(ContainSubstring("Custom changelog entry doesn't follow semantic commit format")))
		})

		It("should accept valid custom changelog format", func() {
			body := `## Motivation
New feature

> Changelog: feat(api): add custom endpoint

## Implementation information
- Added endpoint

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "feat")
			Expect(result.Errors).NotTo(ContainElement(ContainSubstring("Custom changelog")))
		})

		It("should warn on formal language", func() {
			body := `## Motivation
We will utilize this feature to facilitate improvements.

## Implementation information
- Leverage new algorithm

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "feat")
			Expect(result.Warnings).To(ContainElement(ContainSubstring("uses formal language")))
		})

		It("should warn on empty supporting documentation", func() {
			body := `## Motivation
New feature

## Implementation information
- Added endpoint

## Supporting documentation
N/A`

			result := git.ValidatePRBody(body, "feat")
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("Supporting documentation section is empty")))
		})
	})

	Describe("Full Validator", func() {
		It("should pass for valid gh pr create command", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "feat(api): add endpoint" --body "$(cat <<'EOF'
# PR Title

## Motivation

New feature description

## Implementation information

- Added endpoint
- Updated documentation

## Supporting documentation

See docs/api.md
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should fail for invalid title format", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "Add endpoint" --body "$(cat <<'EOF'
# PR Title

## Motivation

New feature description

## Implementation information

- Added endpoint
- Updated documentation

## Supporting documentation

See docs/api.md
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeFalse())
			Expect(result.Message).To(ContainSubstring("PR validation failed"))
			Expect(result.Message).To(ContainSubstring("doesn't follow semantic commit format"))
		})

		It("should fail for feat(ci) title", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "feat(ci): add workflow" --body "$(cat <<'EOF'
# PR Title

## Motivation

CI improvement description

## Implementation information

- Added workflow
- Updated CI configuration

## Supporting documentation

N/A
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeFalse())
			Expect(result.Message).To(ContainSubstring("Use 'ci(...)' not 'feat(ci)'"))
		})

		It("should fail for missing required sections", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "feat(api): add endpoint" --body "$(cat <<'EOF'
## Motivation
New feature
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeFalse())
			Expect(result.Message).To(ContainSubstring("missing '## Implementation information'"))
			Expect(result.Message).To(ContainSubstring("missing '## Supporting documentation'"))
		})

		It("should fail for non-main base without label", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "feat(api): add endpoint" --base "release/1.0" --body "$(cat <<'EOF'
# PR Title

## Motivation

New feature description

## Implementation information

- Added endpoint
- Updated documentation

## Supporting documentation

See docs/api.md
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeFalse())
			Expect(result.Message).To(ContainSubstring("targets 'release/1.0' but missing label"))
		})

		It("should pass for non-main base with matching label", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "feat(api): add endpoint" --base "release/1.0" --label "release/1.0" --body "$(cat <<'EOF'
# PR Title

## Motivation

New feature description

## Implementation information

- Added endpoint
- Updated documentation

## Supporting documentation

See docs/api.md
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should warn for ci type without ci/skip label", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title "ci(workflow): update pipeline" --body "$(cat <<'EOF'
# PR Title

## Motivation

CI improvement description

> Changelog: skip

## Implementation information

- Updated workflow
- Improved pipeline performance

## Supporting documentation

N/A
EOF
)"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(
				result.Passed,
			).To(BeFalse())
			// Warnings return Passed=false with ShouldBlock=false
			Expect(result.Message).To(ContainSubstring("warnings"))
			Expect(result.Message).To(ContainSubstring("ci/skip-test"))
		})

		It("should handle command chains with gh pr create", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `git add . && gh pr create --title "feat(api): add endpoint" --body "# PR Title

## Motivation

New feature description

## Implementation information

- Added endpoint
- Updated documentation

## Supporting documentation

See docs/api.md"`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should extract title with single quotes", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: `gh pr create --title 'feat(api): add endpoint' --body '# PR Title

## Motivation

New feature description

## Implementation information

- Added endpoint
- Updated documentation

## Supporting documentation

See docs/api.md'`,
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		})

		It("should pass for non-gh commands", func() {
			ctx := &hook.Context{
				EventType: hook.PreToolUse,
				ToolName:  hook.Bash,
				ToolInput: hook.ToolInput{
					Command: "git status",
				},
			}

			result := validator.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		})
	})
})
