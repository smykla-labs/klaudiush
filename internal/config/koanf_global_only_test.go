package config

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// newGlobalOnlyLoader creates a loader over a fake $HOME with an empty work dir.
func newGlobalOnlyLoader() (loader *KoanfLoader, homeDir string) {
	homeDir, err := os.MkdirTemp("", "global-only-home-")
	Expect(err).NotTo(HaveOccurred())

	workDir := filepath.Join(homeDir, "projects", "myrepo")
	Expect(os.MkdirAll(workDir, 0o755)).To(Succeed())

	loader, err = NewKoanfLoaderWithDirs(homeDir, workDir)
	Expect(err).NotTo(HaveOccurred())

	return loader, homeDir
}

var _ = Describe("LoadGlobalConfigOnly", func() {
	It("returns nil when no global config exists", func() {
		loader, homeDir := newGlobalOnlyLoader()

		DeferCleanup(func() { os.RemoveAll(homeDir) })

		cfg, path, err := loader.LoadGlobalConfigOnly()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).To(BeNil())
		Expect(path).To(BeEmpty())
	})

	It("reads the global config file the writer targets", func() {
		loader, homeDir := newGlobalOnlyLoader()

		DeferCleanup(func() { os.RemoveAll(homeDir) })

		writeGlobalConfig(homeDir, `version = 1

[validators.git.commit]
enabled = true

[validators.file.markdown]
enabled = false
`)

		cfg, path, err := loader.LoadGlobalConfigOnly()
		Expect(err).NotTo(HaveOccurred())
		Expect(path).To(Equal(loader.GlobalConfigPath()))
		Expect(path).To(Equal(filepath.Join(homeDir, GlobalConfigDir, GlobalConfigFile)))

		// Every section must survive so callers can rewrite the file
		// without dropping what they did not touch.
		Expect(cfg.Validators.Git.Commit.Enabled).NotTo(BeNil())
		Expect(*cfg.Validators.Git.Commit.Enabled).To(BeTrue())
		Expect(cfg.Validators.File.Markdown.Enabled).NotTo(BeNil())
		Expect(*cfg.Validators.File.Markdown.Enabled).To(BeFalse())
	})

	It("ignores project config and defaults", func() {
		loader, homeDir := newGlobalOnlyLoader()

		DeferCleanup(func() { os.RemoveAll(homeDir) })

		writeGlobalConfig(homeDir, "version = 1\n")
		writeConfigAt(
			filepath.Join(homeDir, "projects", "myrepo"),
			"[validators.git.commit]\nenabled = false\n",
		)

		cfg, _, err := loader.LoadGlobalConfigOnly()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Validators).To(BeNil())
	})

	It("defaults the version when the file omits it", func() {
		loader, homeDir := newGlobalOnlyLoader()

		DeferCleanup(func() { os.RemoveAll(homeDir) })

		writeGlobalConfig(homeDir, "[validators.git.commit]\nenabled = true\n")

		cfg, _, err := loader.LoadGlobalConfigOnly()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Version).To(Equal(1))
	})

	It("reports an error for malformed TOML", func() {
		loader, homeDir := newGlobalOnlyLoader()

		DeferCleanup(func() { os.RemoveAll(homeDir) })

		writeGlobalConfig(homeDir, "[validators.git.commit\nenabled = true\n")

		_, _, err := loader.LoadGlobalConfigOnly()
		Expect(err).To(HaveOccurred())
	})
})
