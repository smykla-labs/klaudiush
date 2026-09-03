package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/pkg/config"
)

var _ = Describe("OpenCodeProviderConfig", func() {
	It("creates the provider config on demand", func() {
		providers := &config.ProvidersConfig{}

		Expect(providers.OpenCode).To(BeNil())
		Expect(providers.GetOpenCode()).NotTo(BeNil())
		Expect(providers.OpenCode).NotTo(BeNil())
	})

	// Claude is on by default; every other provider is opt-in.
	It("is disabled unless explicitly enabled", func() {
		Expect((*config.OpenCodeProviderConfig)(nil).IsEnabled()).To(BeFalse())
		Expect((&config.OpenCodeProviderConfig{}).IsEnabled()).To(BeFalse())

		enabled := true
		Expect((&config.OpenCodeProviderConfig{Enabled: &enabled}).IsEnabled()).To(BeTrue())

		disabled := false
		Expect((&config.OpenCodeProviderConfig{Enabled: &disabled}).IsEnabled()).To(BeFalse())
	})

	It("reports whether a plugin path is set", func() {
		Expect((*config.OpenCodeProviderConfig)(nil).HasPluginPath()).To(BeFalse())
		Expect((&config.OpenCodeProviderConfig{}).HasPluginPath()).To(BeFalse())
		Expect((&config.OpenCodeProviderConfig{PluginPath: "/tmp/p.ts"}).HasPluginPath()).
			To(BeTrue())
	})
})
