package github_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/validators/github"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

var _ = Describe("APIValidator", func() {
	var (
		apiValidator *github.APIValidator
		ctx          context.Context
	)

	bashContext := func(command string) *hook.Context {
		return &hook.Context{
			EventType: hook.EventTypePreToolUse,
			ToolName:  hook.ToolTypeBash,
			ToolInput: hook.ToolInput{Command: command},
		}
	}

	BeforeEach(func() {
		apiValidator = github.NewAPIValidator(nil, logger.NewNoOpLogger(), nil)
		ctx = context.Background()
	})

	Describe("commit-creating calls", func() {
		DescribeTable(
			"blocks the call",
			func(command string) {
				result := apiValidator.Validate(ctx, bashContext(command))

				Expect(result.Passed).To(BeFalse())
				Expect(result.ShouldBlock).To(BeTrue())
				Expect(result.Reference.Code()).To(Equal("GH002"))
				Expect(result.FixHint).To(ContainSubstring("git commit -sS"))
			},
			Entry(
				"contents PUT",
				`gh api --method PUT /repos/o/r/contents/README.md -f message=x -f content=y`,
			),
			Entry(
				"contents DELETE",
				`gh api -X DELETE repos/o/r/contents/README.md -f message=x -f sha=abc`,
			),
			Entry(
				"git commits POST",
				`gh api repos/o/r/git/commits -f message=x -f tree=abc`,
			),
			Entry(
				"merges POST",
				`gh api repos/o/r/merges -f base=main -f head=topic`,
			),
			Entry(
				"pull request merge PUT",
				`gh api --method PUT repos/o/r/pulls/42/merge`,
			),
			Entry(
				"full URL form",
				`gh api -X PUT https://api.github.com/repos/o/r/contents/x.txt -f message=x`,
			),
			Entry(
				"repository spelled from variables",
				`gh api -X PUT "repos/$OWNER/$REPO/contents/README.md" -f message=x`,
			),
			Entry(
				"one stage of a pipeline",
				`gh api -X PUT repos/o/r/contents/README.md -f message=x | jq .commit.sha`,
			),
			Entry(
				"one stage of a compound command",
				`git status && gh api -X PUT repos/o/r/contents/README.md -f message=x`,
			),
			Entry(
				"graphql createCommitOnBranch",
				`gh api graphql -f query='mutation { createCommitOnBranch(input: $i) { commit { url } } }'`,
			),
		)

		It("names the method, the endpoint and the intended path", func() {
			result := apiValidator.Validate(
				ctx,
				bashContext(`gh api -X PUT repos/o/r/contents/README.md -f message=x`),
			)

			Expect(result.Message).To(ContainSubstring("PUT repos/o/r/contents/README.md"))
			Expect(result.Details["help"]).To(ContainSubstring("git commit -sS"))
		})
	})

	Describe("calls that create no commit", func() {
		DescribeTable(
			"passes the call",
			func(command string) {
				result := apiValidator.Validate(ctx, bashContext(command))
				Expect(result.Passed).To(BeTrue())
			},
			Entry(
				"branch creation",
				`gh api -X POST repos/o/r/git/refs -f ref=refs/heads/topic -f sha=abc`,
			),
			Entry(
				"reading contents",
				`gh api repos/o/r/contents/README.md`,
			),
			Entry(
				"reading contents through a pipeline",
				`gh api repos/o/r/contents/README.md --jq .sha | cat`,
			),
			Entry(
				"listing workflow runs",
				`gh api repos/o/r/actions/runs`,
			),
			Entry(
				"creating a pull request",
				`gh api -X POST repos/o/r/pulls -f title=x -f head=topic -f base=main`,
			),
			Entry(
				"a harmless graphql query",
				`gh api graphql -f query='query { viewer { login } }'`,
			),
			Entry(
				"gh pr create",
				`gh pr create --title "feat(api): x" --body y`,
			),
			Entry(
				"a plain git commit",
				`git commit -sS -m "feat(api): x"`,
			),
		)
	})

	Describe("clients other than gh", func() {
		DescribeTable(
			"blocks the call",
			func(command string) {
				result := apiValidator.Validate(ctx, bashContext(command))

				Expect(result.Passed).To(BeFalse())
				Expect(result.ShouldBlock).To(BeTrue())
				Expect(result.Reference.Code()).To(Equal("GH002"))
			},
			Entry(
				"curl with an explicit method",
				`curl -X PUT -H "Authorization: bearer $T" `+
					`https://api.github.com/repos/o/r/contents/README.md -d '{"message":"x"}'`,
			),
			Entry(
				"curl with the method attached to the flag",
				`curl -XDELETE https://api.github.com/repos/o/r/contents/README.md`,
			),
			Entry(
				"curl whose method is implied by --upload-file",
				`curl -T body.json https://api.github.com/repos/o/r/contents/README.md`,
			),
			Entry(
				"curl whose method is implied by --data",
				`curl --data '{"base":"main"}' https://api.github.com/repos/o/r/merges`,
			),
			Entry(
				"curl using --url instead of a positional",
				`curl -X PUT --url https://api.github.com/repos/o/r/contents/x`,
			),
			Entry(
				"curl against GitHub Enterprise Server",
				`curl -X PUT https://ghe.example.com/api/v3/repos/o/r/contents/x -d '{}'`,
			),
			Entry(
				"wget",
				`wget --method=PUT --body-data='{"message":"x"}' `+
					`https://api.github.com/repos/o/r/contents/README.md`,
			),
			Entry(
				"httpie",
				`http PUT https://api.github.com/repos/o/r/contents/README.md message=x`,
			),
			Entry(
				"xh",
				`xh DELETE https://api.github.com/repos/o/r/contents/README.md`,
			),
			Entry(
				"curl posting a graphql mutation",
				`curl -X POST https://api.github.com/graphql `+
					`-d '{"query":"mutation { createCommitOnBranch(input: $i) { url } }"}'`,
			),
			Entry(
				"an inline octokit script",
				`node -e 'octokit.rest.repos.createOrUpdateFileContents({owner, repo, path})'`,
			),
			Entry(
				"an inline octokit request call",
				`node -e 'await octokit.request("PUT /repos/{owner}/{repo}/contents/{path}", opts)'`,
			),
			Entry(
				"an inline python request",
				`python3 -c 'requests.put("https://api.github.com/repos/o/r/contents/x", json=b)'`,
			),
			Entry(
				"curl with a token in the userinfo",
				`curl -X PUT https://x-access-token:$T@api.github.com/repos/o/r/contents/x -d '{}'`,
			),
			Entry(
				"curl with an explicit port",
				`curl -X PUT https://api.github.com:443/repos/o/r/contents/x -d '{}'`,
			),
			Entry(
				"curl with an upper-case host",
				`curl -X PUT https://API.GITHUB.COM/repos/o/r/contents/x -d '{}'`,
			),
			Entry(
				"curl with an upper-case scheme",
				`curl -X PUT HTTPS://api.github.com/repos/o/r/contents/x -d '{}'`,
			),
			Entry(
				"curl with no scheme at all",
				`curl -X PUT api.github.com/repos/o/r/contents/x -d '{}'`,
			),
			Entry(
				"curl fetching a second URL",
				`curl -X PUT https://example.com/ok https://api.github.com/repos/o/r/contents/x`,
			),
			Entry(
				"curl with the call in a --next request",
				`curl https://example.com/ok --next -X PUT `+
					`https://api.github.com/repos/o/r/contents/x -d '{}'`,
			),
		)

		DescribeTable(
			"passes the call",
			func(command string) {
				result := apiValidator.Validate(ctx, bashContext(command))
				Expect(result.Passed).To(BeTrue())
			},
			Entry(
				"curl reading contents",
				`curl https://api.github.com/repos/o/r/contents/README.md`,
			),
			Entry(
				"curl creating a branch ref",
				`curl -X POST https://api.github.com/repos/o/r/git/refs -d '{"ref":"refs/heads/x"}'`,
			),
			Entry(
				"curl against a host that is not GitHub",
				`curl -X PUT https://example.com/repos/o/r/contents/README.md -d '{}'`,
			),
			Entry(
				"curl downloading a release asset",
				`curl -L -o out.tgz https://github.com/o/r/archive/refs/tags/v1.tar.gz`,
			),
			Entry(
				"curl reading GitHub then writing elsewhere",
				`curl https://api.github.com/repos/o/r/contents/x --next -X PUT https://example.com/ok -d '{}'`,
			),
			Entry(
				"an unrelated route in a script",
				`node -e 'app.put("/repos/:id/contents/:path", handler)'`,
			),
			Entry(
				"a script mentioning a blocked call without invoking it",
				`echo "use repos.createOrUpdateFileContents instead of git"`,
			),
			Entry(
				"curl posting a harmless graphql query",
				`curl -X POST https://api.github.com/graphql -d '{"query":"query { viewer }"}'`,
			),
		)

		It("names the tool that sends the request", func() {
			result := apiValidator.Validate(ctx, bashContext(
				`curl -X PUT https://api.github.com/repos/o/r/contents/x -d '{}'`,
			))

			Expect(result.Message).To(HavePrefix("curl PUT repos/o/r/contents/x"))
		})

		It("leaves other clients alone when the check is turned off", func() {
			disabled := false
			cfg := &config.APIValidatorConfig{CheckHTTPClients: &disabled}
			apiValidator = github.NewAPIValidator(cfg, logger.NewNoOpLogger(), nil)

			viaCurl := apiValidator.Validate(ctx, bashContext(
				`curl -X PUT https://api.github.com/repos/o/r/contents/x -d '{}'`,
			))
			Expect(viaCurl.Passed).To(BeTrue())

			viaGH := apiValidator.Validate(ctx, bashContext(
				`gh api -X PUT repos/o/r/contents/x -f message=y`,
			))
			Expect(viaGH.Passed).To(BeFalse())
		})
	})

	Describe("endpoints built from variables", func() {
		It("resolves an assignment made on the same command line", func() {
			result := apiValidator.Validate(ctx, bashContext(
				`EP=repos/o/r/contents/README.md; gh api -X PUT "$EP" -f message=x`,
			))

			Expect(result.Passed).To(BeFalse())
			Expect(result.Reference.Code()).To(Equal("GH002"))
		})

		It("resolves an assignment used as a command prefix", func() {
			result := apiValidator.Validate(ctx, bashContext(
				`EP=repos/o/r/merges gh api "$EP" -f base=main -f head=topic`,
			))

			Expect(result.Passed).To(BeFalse())
			Expect(result.Reference.Code()).To(Equal("GH002"))
		})

		It("blocks a write whose endpoint cannot be resolved", func() {
			result := apiValidator.Validate(
				ctx,
				bashContext(`gh api -X PUT "$ENDPOINT" -f message=x`),
			)

			Expect(result.Passed).To(BeFalse())
			Expect(result.ShouldBlock).To(BeTrue())
			Expect(result.Reference.Code()).To(Equal("GH003"))
		})

		It("allows a read whose endpoint cannot be resolved", func() {
			result := apiValidator.Validate(ctx, bashContext(`gh api "$ENDPOINT" --jq .sha`))
			Expect(result.Passed).To(BeTrue())
		})

		It("allows a resolved write to an endpoint that creates no commit", func() {
			result := apiValidator.Validate(ctx, bashContext(
				`EP=repos/o/r/git/refs; gh api -X POST "$EP" -f ref=refs/heads/topic`,
			))
			Expect(result.Passed).To(BeTrue())
		})
	})

	Describe("GraphQL body in a file", func() {
		It("reads a file written earlier in the same command line", func() {
			result := apiValidator.Validate(ctx, bashContext(
				"cat > query.json <<'EOF'\n"+
					`{"query": "mutation { createCommitOnBranch(input: $i) { commit { url } } }"}`+
					"\nEOF\ngh api graphql --input query.json",
			))

			Expect(result.Passed).To(BeFalse())
			Expect(result.Reference.Code()).To(Equal("GH002"))
		})

		It("reads a file that exists on disk", func() {
			dir := GinkgoT().TempDir()
			path := filepath.Join(dir, "query.json")
			Expect(os.WriteFile(
				path,
				[]byte(`{"query": "mutation { createCommitOnBranch(input: $i) { url } }"}`),
				0o600,
			)).To(Succeed())

			result := apiValidator.Validate(ctx, bashContext(`gh api graphql --input `+path))

			Expect(result.Passed).To(BeFalse())
			Expect(result.Reference.Code()).To(Equal("GH002"))
		})

		It("passes a file on disk carrying a harmless query", func() {
			dir := GinkgoT().TempDir()
			path := filepath.Join(dir, "query.json")
			Expect(os.WriteFile(
				path,
				[]byte(`{"query": "query { viewer { login } }"}`),
				0o600,
			)).To(Succeed())

			result := apiValidator.Validate(ctx, bashContext(`gh api graphql --input `+path))
			Expect(result.Passed).To(BeTrue())
		})

		It("blocks when the file cannot be read", func() {
			result := apiValidator.Validate(ctx, bashContext(`gh api graphql --input missing.json`))

			Expect(result.Passed).To(BeFalse())
			Expect(result.Reference.Code()).To(Equal("GH003"))
		})
	})

	Describe("configuration", func() {
		It("allows unverifiable writes when the check is turned off", func() {
			disabled := false
			cfg := &config.APIValidatorConfig{BlockUnverifiableCalls: &disabled}
			apiValidator = github.NewAPIValidator(cfg, logger.NewNoOpLogger(), nil)

			opaque := apiValidator.Validate(
				ctx,
				bashContext(`gh api -X PUT "$ENDPOINT" -f message=x`),
			)
			Expect(opaque.Passed).To(BeTrue())

			unreadable := apiValidator.Validate(
				ctx,
				bashContext(`gh api graphql --input missing.json`),
			)
			Expect(unreadable.Passed).To(BeTrue())
		})

		It("uses the configured endpoint rules instead of the defaults", func() {
			cfg := &config.APIValidatorConfig{
				BlockedEndpoints: []string{"POST **/git/refs"},
			}
			apiValidator = github.NewAPIValidator(cfg, logger.NewNoOpLogger(), nil)

			blocked := apiValidator.Validate(
				ctx,
				bashContext(`gh api -X POST repos/o/r/git/refs -f ref=refs/heads/topic`),
			)
			Expect(blocked.Passed).To(BeFalse())

			allowed := apiValidator.Validate(
				ctx,
				bashContext(`gh api -X PUT repos/o/r/contents/README.md -f message=x`),
			)
			Expect(allowed.Passed).To(BeTrue())
		})

		It("supports a wildcard method", func() {
			cfg := &config.APIValidatorConfig{
				BlockedEndpoints: []string{"* **/contents/**"},
			}
			apiValidator = github.NewAPIValidator(cfg, logger.NewNoOpLogger(), nil)

			result := apiValidator.Validate(ctx, bashContext(`gh api repos/o/r/contents/README.md`))
			Expect(result.Passed).To(BeFalse())
		})

		It("skips a rule with an invalid pattern instead of failing the hook", func() {
			cfg := &config.APIValidatorConfig{
				BlockedEndpoints: []string{"PUT [", "PUT **/contents/**"},
			}
			apiValidator = github.NewAPIValidator(cfg, logger.NewNoOpLogger(), nil)

			result := apiValidator.Validate(
				ctx,
				bashContext(`gh api -X PUT repos/o/r/contents/README.md`),
			)
			Expect(result.Passed).To(BeFalse())
		})
	})
})
