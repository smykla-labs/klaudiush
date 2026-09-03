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

		// Asserting on registration rather than on any mention of the name:
		// every forwarded event also appears as an invoke() argument, so a
		// substring check would pass even with all subscriptions deleted.
		It("subscribes to every advertised opencode event", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())

			source := string(rendered)

			for _, eventName := range hook.OpenCodeEventNames() {
				subscribed := strings.Contains(source, `"`+eventName+`":`) ||
					strings.Contains(source, `"experimental.`+eventName+`":`) ||
					strings.Contains(source, `case "`+eventName+`":`)

				Expect(subscribed).To(
					BeTrue(),
					"event %s is named but not subscribed to", eventName,
				)
			}
		})

		// permission.ask fires for a subset of calls that tool.execute.before
		// already covers; registering both validates one call twice.
		It("does not register the redundant approval hook", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).NotTo(ContainSubstring(`"permission.ask":`))
		})

		// A silent fail-open is indistinguishable from a clean pass.
		It("reports invocation failures on stderr", func() {
			rendered, err := settings.RenderOpenCodePlugin(binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring("console.error"))
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

	Describe("defaults", func() {
		It("places the plugin in the opencode config directory", func() {
			Expect(settings.DefaultOpenCodePluginPath()).
				To(HaveSuffix(".config/opencode/plugin/klaudiush.ts"))
		})

		It("re-exports the forwarded event list", func() {
			Expect(settings.OpenCodeEventNames()).To(Equal(hook.OpenCodeEventNames()))
			Expect(settings.OpenCodeEventNames()).NotTo(ContainElement("permission.ask"))
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

		// The compaction hook is registered behind opencode's experimental
		// prefix, and four events arrive on the shared bus as case labels, so
		// the checker must recognise all three subscription forms.
		It("does not credit an event that is only mentioned", func() {
			_, err := settings.InstallOpenCodeDispatcher(pluginPath, binaryPath)
			Expect(err).NotTo(HaveOccurred())

			parser := settings.NewOpenCodePluginParser(pluginPath)

			hasHook, err := parser.HasEventHook("permission.ask", binaryPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(hasHook).To(BeFalse())
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
