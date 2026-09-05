package parser_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/pkg/parser"
)

var _ = Describe("ParseHTTPClientCommand", func() {
	parseFirst := func(command, name string) parser.Command {
		GinkgoHelper()

		result, err := parser.NewBashParser().Parse(command)
		Expect(err).NotTo(HaveOccurred())

		commands := result.GetCommands(name)
		Expect(commands).NotTo(BeEmpty())

		return commands[0]
	}

	DescribeTable(
		"resolves the method and URL",
		func(command, tool, wantMethod, wantURL string) {
			req, ok := parser.ParseHTTPClientCommand(parseFirst(command, tool))
			Expect(ok).To(BeTrue())
			Expect(req.Method).To(Equal(wantMethod))
			Expect(req.URL).To(Equal(wantURL))
		},
		Entry(
			"curl defaults to GET",
			"curl https://api.github.com/repos/o/r/contents/x",
			"curl", "GET", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"curl -X before the URL",
			"curl -X PUT https://api.github.com/repos/o/r/contents/x",
			"curl", "PUT", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"curl -X after the URL",
			"curl https://api.github.com/repos/o/r/contents/x -X DELETE",
			"curl", "DELETE", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"curl with the method attached",
			"curl -XPATCH https://api.github.com/repos/o/r/contents/x",
			"curl", "PATCH", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"curl --data implies POST",
			`curl --data '{"a":1}' https://api.github.com/repos/o/r/merges`,
			"curl", "POST", "https://api.github.com/repos/o/r/merges",
		),
		Entry(
			"curl --upload-file implies PUT",
			"curl -T body.json https://api.github.com/repos/o/r/contents/x",
			"curl", "PUT", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"curl --url carries the URL",
			"curl -X PUT --url https://api.github.com/repos/o/r/contents/x",
			"curl", "PUT", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"curl header value is not mistaken for the URL",
			`curl -H "Accept: application/json" https://api.github.com/repos/o/r`,
			"curl", "GET", "https://api.github.com/repos/o/r",
		),
		Entry(
			"wget --method",
			"wget --method=PUT --body-data=x https://api.github.com/repos/o/r/contents/x",
			"wget", "PUT", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"wget --post-data implies POST",
			"wget --post-data=x https://api.github.com/repos/o/r/merges",
			"wget", "POST", "https://api.github.com/repos/o/r/merges",
		),
		Entry(
			"httpie takes the method as a positional",
			"http PUT https://api.github.com/repos/o/r/contents/x message=y",
			"http", "PUT", "https://api.github.com/repos/o/r/contents/x",
		),
		Entry(
			"httpie request items imply POST",
			"http https://api.github.com/repos/o/r/merges base=main head=topic",
			"http", "POST", "https://api.github.com/repos/o/r/merges",
		),
		Entry(
			"xh takes the method as a positional",
			"xh DELETE https://api.github.com/repos/o/r/contents/x",
			"xh", "DELETE", "https://api.github.com/repos/o/r/contents/x",
		),
	)

	It("records an inline body", func() {
		req, ok := parser.ParseHTTPClientCommand(parseFirst(
			`curl -X POST https://api.github.com/graphql -d '{"query":"mutation {}"}'`, "curl",
		))
		Expect(ok).To(BeTrue())
		Expect(req.Body).To(ContainSubstring("mutation"))
	})

	It("follows curl's @path form to a body file", func() {
		req, ok := parser.ParseHTTPClientCommand(parseFirst(
			"curl -X POST https://api.github.com/graphql -d @query.json", "curl",
		))
		Expect(ok).To(BeTrue())
		Expect(req.BodyFile).To(Equal("query.json"))
	})

	It("reports no request when the command carries no URL", func() {
		_, ok := parser.ParseHTTPClientCommand(parseFirst("curl --version", "curl"))
		Expect(ok).To(BeFalse())
	})

	It("reports no request for a command that is not a client", func() {
		_, ok := parser.ParseHTTPClientCommand(
			parser.Command{Name: "git", Args: []string{"status"}},
		)
		Expect(ok).To(BeFalse())
	})

	Describe("SplitURL", func() {
		DescribeTable(
			"separates host from path",
			func(raw, wantHost, wantPath string) {
				host, path := parser.SplitURL(raw)
				Expect(host).To(Equal(wantHost))
				Expect(path).To(Equal(wantPath))
			},
			Entry(
				"https URL",
				"https://api.github.com/repos/o/r", "api.github.com", "/repos/o/r",
			),
			Entry(
				"http URL",
				"http://ghe.example.com/api/v3/repos/o/r", "ghe.example.com", "/api/v3/repos/o/r",
			),
			Entry("host with no path", "https://api.github.com", "api.github.com", ""),
			Entry("bare path", "repos/o/r/contents/x", "", "repos/o/r/contents/x"),
		)
	})

	Describe("FindAPICallsInText", func() {
		It("finds an octokit request call and marks it explicit", func() {
			calls := parser.FindAPICallsInText(
				`await octokit.request("PUT /repos/{owner}/{repo}/contents/{path}", opts)`,
			)
			Expect(calls).To(HaveLen(1))
			Expect(calls[0].Method).To(Equal("PUT"))
			Expect(calls[0].URL).To(Equal("/repos/{owner}/{repo}/contents/{path}"))
			Expect(calls[0].ExplicitAPICall).To(BeTrue())
		})

		It("finds a verb-named client method and leaves it implicit", func() {
			calls := parser.FindAPICallsInText(
				`axios.put("https://api.github.com/repos/o/r/contents/x", body)`,
			)
			Expect(calls).To(HaveLen(1))
			Expect(calls[0].Method).To(Equal("PUT"))
			Expect(calls[0].ExplicitAPICall).To(BeFalse())
		})

		It("finds nothing in text with no call", func() {
			Expect(parser.FindAPICallsInText("PUT /repos/o/r/contents/x is blocked")).To(BeEmpty())
		})
	})

	Describe("FindCallsInText", func() {
		calls := []string{"repos.createOrUpdateFileContents", "pulls.merge"}

		It("matches a name in call form", func() {
			Expect(
				parser.FindCallsInText(`octokit.rest.repos.createOrUpdateFileContents({})`, calls),
			).
				To(Equal("repos.createOrUpdateFileContents"))
		})

		It("ignores a name that is only mentioned", func() {
			Expect(
				parser.FindCallsInText("see repos.createOrUpdateFileContents", calls),
			).To(BeEmpty())
		})
	})
})
