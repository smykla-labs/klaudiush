package parser_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/pkg/parser"
)

var _ = Describe("Assignments", func() {
	parse := func(command string) *parser.ParseResult {
		GinkgoHelper()

		result, err := parser.NewBashParser().Parse(command)
		Expect(err).NotTo(HaveOccurred())

		return result
	}

	It("records a standalone assignment", func() {
		result := parse(`REPO=owner/name; echo done`)
		Expect(result.Assignments).To(HaveKeyWithValue("REPO", "owner/name"))
	})

	It("records an assignment used as a command prefix", func() {
		result := parse(`REPO=owner/name gh api "repos/$REPO"`)
		Expect(result.Assignments).To(HaveKeyWithValue("REPO", "owner/name"))
	})

	It("skips an append assignment, whose full value is unknown", func() {
		result := parse(`PATHS+=/extra; echo done`)
		Expect(result.Assignments).NotTo(HaveKey("PATHS"))
	})

	Describe("ExpandVars", func() {
		It("substitutes a known assignment", func() {
			result := parse(`REPO=owner/name; echo done`)
			Expect(result.ExpandVars("repos/${REPO}/contents/x")).
				To(Equal("repos/owner/name/contents/x"))
		})

		It("resolves an assignment that references another", func() {
			result := parse(`OWNER=acme; REPO=${OWNER}/widget; echo done`)
			Expect(result.ExpandVars("repos/${REPO}")).To(Equal("repos/acme/widget"))
		})

		It("leaves an unknown reference in place", func() {
			result := parse(`OTHER=x; echo done`)
			Expect(result.ExpandVars("repos/${REPO}")).To(Equal("repos/${REPO}"))
			Expect(parser.HasUnresolvedVars(result.ExpandVars("repos/${REPO}"))).To(BeTrue())
		})

		It("terminates on a self-referential assignment", func() {
			result := parse(`LOOP=${LOOP}/x; echo done`)
			Expect(result.ExpandVars("${LOOP}")).To(ContainSubstring("${LOOP}"))
		})

		It("returns the input unchanged when nothing was assigned", func() {
			result := parse(`echo done`)
			Expect(result.ExpandVars("repos/${REPO}")).To(Equal("repos/${REPO}"))
		})
	})
})
