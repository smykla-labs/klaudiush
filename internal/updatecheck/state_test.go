package updatecheck_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/updatecheck"
)

var _ = Describe("State", func() {
	var tmpDir string

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
	})

	Describe("LoadState", func() {
		It("returns zero state when file does not exist", func() {
			state := updatecheck.LoadState(filepath.Join(tmpDir, "nonexistent.json"))
			Expect(state.LastChecked).To(BeZero())
			Expect(state.LatestVersion).To(BeEmpty())
			Expect(state.NotifiedVersion).To(BeEmpty())
		})

		It("returns zero state when file is corrupt", func() {
			path := filepath.Join(tmpDir, "corrupt.json")
			Expect(os.WriteFile(path, []byte("not json"), 0o600)).To(Succeed())

			state := updatecheck.LoadState(path)
			Expect(state.LastChecked).To(BeZero())
		})

		It("parses a valid state file", func() {
			path := filepath.Join(tmpDir, "state.json")
			content := `{"last_checked":"2026-03-23T10:00:00Z","latest_version":"v1.31.0","notified_version":"v1.30.0"}`
			Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())

			state := updatecheck.LoadState(path)
			Expect(state.LatestVersion).To(Equal("v1.31.0"))
			Expect(state.NotifiedVersion).To(Equal("v1.30.0"))
			Expect(state.LastChecked).NotTo(BeZero())
		})
	})

	Describe("SaveState", func() {
		It("roundtrips state correctly", func() {
			path := filepath.Join(tmpDir, "state.json")
			now := time.Now().Truncate(time.Second)
			original := &updatecheck.State{
				LastChecked:     now,
				LatestVersion:   "v2.0.0",
				NotifiedVersion: "v2.0.0",
			}

			Expect(updatecheck.SaveState(path, original)).To(Succeed())

			loaded := updatecheck.LoadState(path)
			Expect(loaded.LatestVersion).To(Equal("v2.0.0"))
			Expect(loaded.NotifiedVersion).To(Equal("v2.0.0"))
			Expect(loaded.LastChecked.Unix()).To(Equal(now.Unix()))
		})

		It("creates parent directories", func() {
			path := filepath.Join(tmpDir, "nested", "deep", "state.json")
			state := &updatecheck.State{LatestVersion: "v1.0.0"}

			Expect(updatecheck.SaveState(path, state)).To(Succeed())

			loaded := updatecheck.LoadState(path)
			Expect(loaded.LatestVersion).To(Equal("v1.0.0"))
		})
	})
})
