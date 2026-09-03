package settings_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/doctor/settings"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

var _ = Describe("opencode bridge plugin", func() {
	const binaryPath = "/opt/homebrew/bin/klaudiush"

	var pluginPath string

	BeforeEach(func() {
		pluginPath = filepath.Join(GinkgoT().TempDir(), "plugin", "klaudiush.ts")
	})

	Describe("RenderOpenCodePlugin", func() {
		It("embeds the resolved binary path", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring(binaryPath))
		})

		It("leaves no unrendered template directives", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).NotTo(ContainSubstring("{{"))
		})

		It("forwards every advertised opencode event", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())

			for _, eventName := range hook.OpenCodeEventNames() {
				Expect(string(rendered)).To(
					ContainSubstring(`"`+eventName+`"`),
					"event %s missing from plugin", eventName,
				)
			}
		})

		It("passes the opencode provider on the command line", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring(`"--provider", "opencode"`))
		})

		// opencode calls every export of a plugin file as a plugin, so a stray
		// exported helper breaks loading for the whole file.
		It("exports exactly one symbol", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())

			exports := 0

			for line := range strings.SplitSeq(string(rendered), "\n") {
				if strings.HasPrefix(line, "export") {
					exports++
				}
			}

			Expect(exports).To(Equal(1))
		})
	})

	Describe("InstallOpenCodeDispatcher", func() {
		It("creates the plugin and reports it as newly written", func() {
			alreadyInstalled, err := settings.InstallOpenCodeDispatcher(pluginPath, binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(alreadyInstalled).To(BeFalse())

			content, readErr := os.ReadFile(pluginPath)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(binaryPath))
		})

		It("is idempotent", func() {
			_, err := settings.InstallOpenCodeDispatcher(pluginPath, binaryPath)
			Expect(err).NotTo(HaveOccurred())

			alreadyInstalled, err := settings.InstallOpenCodeDispatcher(pluginPath, binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(alreadyInstalled).To(BeTrue())
		})

		It("rewrites a stale plugin when the binary path changes", func() {
			_, err := settings.InstallOpenCodeDispatcher(pluginPath, "/usr/local/bin/klaudiush")
			Expect(err).NotTo(HaveOccurred())

			alreadyInstalled, err := settings.InstallOpenCodeDispatcher(pluginPath, binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(alreadyInstalled).To(BeFalse())

			content, readErr := os.ReadFile(pluginPath)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring(binaryPath))
			Expect(string(content)).NotTo(ContainSubstring("/usr/local/bin/klaudiush"))
		})
	})

	Describe("OpenCodePluginParser", func() {
		It("reports a missing plugin as unregistered rather than failing", func() {
			parser := settings.NewOpenCodePluginParser(pluginPath)

			registered, err := parser.IsDispatcherRegistered(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(registered).To(BeFalse())
		})

		It("surfaces a missing plugin from Read", func() {
			_, err := settings.NewOpenCodePluginParser(pluginPath).Read()
			Expect(err).To(MatchError(settings.ErrPluginNotFound))
		})

		It("detects the installed dispatcher and its events", func() {
			_, err := settings.InstallOpenCodeDispatcher(pluginPath, binaryPath)
			Expect(err).NotTo(HaveOccurred())

			parser := settings.NewOpenCodePluginParser(pluginPath)

			registered, err := parser.IsDispatcherRegistered(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(registered).To(BeTrue())

			for _, eventName := range hook.OpenCodeEventNames() {
				hasHook, hookErr := parser.HasEventHook(eventName, binaryPath)
				Expect(hookErr).NotTo(HaveOccurred())
				Expect(hasHook).To(BeTrue(), "event %s not detected", eventName)
			}
		})

		It("does not credit a plugin that calls a different binary", func() {
			_, err := settings.InstallOpenCodeDispatcher(pluginPath, "/somewhere/else/klaudiush")
			Expect(err).NotTo(HaveOccurred())

			parser := settings.NewOpenCodePluginParser(pluginPath)

			registered, err := parser.IsDispatcherRegistered(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(registered).To(BeFalse())

			hasHook, err := parser.HasEventHook("tool.execute.before", binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(hasHook).To(BeFalse())
		})
	})
})
