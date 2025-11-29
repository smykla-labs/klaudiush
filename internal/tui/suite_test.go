package tui_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTui(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TUI Suite")
}
