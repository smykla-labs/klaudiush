package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/github"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

var _ = Describe("checkForUpdates", func() {
	var (
		originalVersion         string
		originalNewGitHubClient func() github.Client
	)

	BeforeEach(func() {
		originalVersion = version
		originalNewGitHubClient = newGitHubClient
	})

	AfterEach(func() {
		version = originalVersion
		newGitHubClient = originalNewGitHubClient
	})

	It("skips GitHub client creation for dev builds", func() {
		version = "dev"
		clientCreated := false
		newGitHubClient = func() github.Client {
			clientCreated = true
			return nil
		}

		enabled := true
		cfg := &config.Config{
			UpdateCheck: &config.UpdateCheckConfig{Enabled: &enabled},
		}

		msg := checkForUpdates(cfg, logger.NewNoOpLogger())
		Expect(msg).To(BeEmpty())
		Expect(clientCreated).To(BeFalse())
	})

	It("skips GitHub client creation when update checks are disabled", func() {
		version = "1.31.0"
		clientCreated := false
		newGitHubClient = func() github.Client {
			clientCreated = true
			return nil
		}

		enabled := false
		cfg := &config.Config{
			UpdateCheck: &config.UpdateCheckConfig{Enabled: &enabled},
		}

		msg := checkForUpdates(cfg, logger.NewNoOpLogger())
		Expect(msg).To(BeEmpty())
		Expect(clientCreated).To(BeFalse())
	})
})
