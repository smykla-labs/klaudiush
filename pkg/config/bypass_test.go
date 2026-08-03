package config_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/pkg/config"
)

var _ = Describe("BypassPermissionsConfig", func() {
	trueValue := true
	falseValue := false

	Describe("IsSkipValidation", func() {
		It("returns false for nil config", func() {
			var cfg *config.BypassPermissionsConfig
			Expect(cfg.IsSkipValidation()).To(BeFalse())
		})

		It("returns false when unset", func() {
			cfg := &config.BypassPermissionsConfig{}
			Expect(cfg.IsSkipValidation()).To(BeFalse())
		})

		It("returns false when explicitly disabled", func() {
			cfg := &config.BypassPermissionsConfig{SkipValidation: &falseValue}
			Expect(cfg.IsSkipValidation()).To(BeFalse())
		})

		It("returns true when enabled without expiry", func() {
			cfg := &config.BypassPermissionsConfig{SkipValidation: &trueValue}
			Expect(cfg.IsSkipValidation()).To(BeTrue())
		})

		It("returns true when the expiry is in the future", func() {
			cfg := &config.BypassPermissionsConfig{
				SkipValidation: &trueValue,
				ExpiresAt:      time.Now().Add(time.Hour).Format(time.RFC3339),
			}
			Expect(cfg.IsSkipValidation()).To(BeTrue())
		})

		It("returns false when the expiry has passed", func() {
			cfg := &config.BypassPermissionsConfig{
				SkipValidation: &trueValue,
				ExpiresAt:      time.Now().Add(-time.Hour).Format(time.RFC3339),
			}
			Expect(cfg.IsSkipValidation()).To(BeFalse())
		})

		It("returns true when the expiry cannot be parsed", func() {
			cfg := &config.BypassPermissionsConfig{
				SkipValidation: &trueValue,
				ExpiresAt:      "not-a-timestamp",
			}
			Expect(cfg.IsSkipValidation()).To(BeTrue())
		})
	})

	Describe("IsNotifyEnabled", func() {
		It("defaults to true for nil config", func() {
			var cfg *config.BypassPermissionsConfig
			Expect(cfg.IsNotifyEnabled()).To(BeTrue())
		})

		It("defaults to true when unset", func() {
			cfg := &config.BypassPermissionsConfig{}
			Expect(cfg.IsNotifyEnabled()).To(BeTrue())
		})

		It("returns false when disabled", func() {
			cfg := &config.BypassPermissionsConfig{Notify: &falseValue}
			Expect(cfg.IsNotifyEnabled()).To(BeFalse())
		})
	})

	Describe("GetModes", func() {
		It("returns nil for nil config", func() {
			var cfg *config.BypassPermissionsConfig
			Expect(cfg.GetModes()).To(BeNil())
		})

		It("returns nil when no modes are configured", func() {
			cfg := &config.BypassPermissionsConfig{}
			Expect(cfg.GetModes()).To(BeNil())
		})

		It("returns the configured modes", func() {
			cfg := &config.BypassPermissionsConfig{Modes: []string{"dontAsk"}}
			Expect(cfg.GetModes()).To(Equal([]string{"dontAsk"}))
		})

		It("splits comma-separated entries from environment variables", func() {
			cfg := &config.BypassPermissionsConfig{
				Modes: []string{"dontAsk, bypassPermissions", "yolo"},
			}
			Expect(cfg.GetModes()).To(Equal([]string{"dontAsk", "bypassPermissions", "yolo"}))
		})

		It("drops empty entries", func() {
			cfg := &config.BypassPermissionsConfig{Modes: []string{"", " , "}}
			Expect(cfg.GetModes()).To(BeEmpty())
		})
	})

	Describe("GetBypassPermissions", func() {
		It("creates the section when missing", func() {
			cfg := &config.Config{}
			Expect(cfg.GetBypassPermissions()).NotTo(BeNil())
			Expect(cfg.BypassPermissions).NotTo(BeNil())
		})

		It("returns the existing section", func() {
			existing := &config.BypassPermissionsConfig{Reason: "spike"}
			cfg := &config.Config{BypassPermissions: existing}
			Expect(cfg.GetBypassPermissions()).To(BeIdenticalTo(existing))
		})
	})
})
