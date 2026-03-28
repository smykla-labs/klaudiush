package updatecheck_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/updatecheck"
)

var _ = Describe("FormatNotification", func() {
	It("formats with v-prefix versions", func() {
		msg := updatecheck.FormatNotification("v1.30.1", "v1.31.0")
		Expect(msg).To(Equal(
			"Update available: klaudiush 1.30.1 -> 1.31.0. Run 'klaudiush update' to install.",
		))
	})

	It("formats without v-prefix versions", func() {
		msg := updatecheck.FormatNotification("1.30.1", "1.31.0")
		Expect(msg).To(Equal(
			"Update available: klaudiush 1.30.1 -> 1.31.0. Run 'klaudiush update' to install.",
		))
	})

	It("handles mixed prefix formats", func() {
		msg := updatecheck.FormatNotification("v1.0.0", "2.0.0")
		Expect(msg).To(Equal(
			"Update available: klaudiush 1.0.0 -> 2.0.0. Run 'klaudiush update' to install.",
		))
	})
})
