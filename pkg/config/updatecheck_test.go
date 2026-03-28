package config_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/pkg/config"
)

var _ = Describe("UpdateCheckConfig", func() {
	Describe("IsEnabled", func() {
		It("returns true when nil", func() {
			var c *config.UpdateCheckConfig
			Expect(c.IsEnabled()).To(BeTrue())
		})

		It("returns true when Enabled is nil", func() {
			c := &config.UpdateCheckConfig{}
			Expect(c.IsEnabled()).To(BeTrue())
		})

		It("returns configured value", func() {
			c := &config.UpdateCheckConfig{Enabled: new(bool)}
			Expect(c.IsEnabled()).To(BeFalse())
		})
	})

	Describe("GetCheckInterval", func() {
		It("returns default when nil", func() {
			var c *config.UpdateCheckConfig
			Expect(c.GetCheckInterval()).To(Equal(24 * time.Hour))
		})

		It("returns default when zero", func() {
			c := &config.UpdateCheckConfig{}
			Expect(c.GetCheckInterval()).To(Equal(24 * time.Hour))
		})

		It("returns configured value", func() {
			c := &config.UpdateCheckConfig{
				CheckInterval: config.Duration(6 * time.Hour),
			}
			Expect(c.GetCheckInterval()).To(Equal(6 * time.Hour))
		})
	})

	Describe("IsNotifyEnabled", func() {
		It("returns true when nil", func() {
			var c *config.UpdateCheckConfig
			Expect(c.IsNotifyEnabled()).To(BeTrue())
		})

		It("returns true when Notify is nil", func() {
			c := &config.UpdateCheckConfig{}
			Expect(c.IsNotifyEnabled()).To(BeTrue())
		})

		It("returns configured value", func() {
			c := &config.UpdateCheckConfig{Notify: new(bool)}
			Expect(c.IsNotifyEnabled()).To(BeFalse())
		})
	})
})
