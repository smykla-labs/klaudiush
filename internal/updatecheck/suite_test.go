package updatecheck_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUpdateCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpdateCheck Suite")
}
