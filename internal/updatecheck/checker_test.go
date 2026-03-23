package updatecheck_test

import (
	"context"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/github"
	"github.com/smykla-skalski/klaudiush/internal/updatecheck"
	"github.com/smykla-skalski/klaudiush/pkg/config"
)

// mockClient implements github.Client for testing.
type mockClient struct {
	latestRelease *github.Release
	latestErr     error
	callCount     int
}

func (m *mockClient) GetLatestRelease(_ context.Context, _, _ string) (*github.Release, error) {
	m.callCount++

	return m.latestRelease, m.latestErr
}

func (*mockClient) GetReleaseByTag(_ context.Context, _, _, _ string) (*github.Release, error) {
	return nil, nil //nolint:nilnil // test stub
}

func (*mockClient) GetTags(_ context.Context, _, _ string) ([]*github.Tag, error) {
	return nil, nil
}

func (*mockClient) IsAuthenticated() bool {
	return false
}

var _ = Describe("Checker", func() {
	var (
		tmpDir string
		client *mockClient
		now    time.Time
		nowFn  func() time.Time
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		client = &mockClient{
			latestRelease: &github.Release{TagName: "v2.0.0"},
		}
		now = time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
		nowFn = func() time.Time { return now }
	})

	statePath := func() string {
		return filepath.Join(tmpDir, "update_check.json")
	}

	newChecker := func(version string, cfg *config.UpdateCheckConfig) *updatecheck.Checker {
		return updatecheck.NewChecker(
			version,
			client,
			cfg,
			updatecheck.WithStatePath(statePath()),
			updatecheck.WithTimeFunc(nowFn),
		)
	}

	It("skips dev builds", func() {
		c := newChecker("dev", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())
		Expect(client.callCount).To(Equal(0))
	})

	It("skips when disabled", func() {
		c := newChecker("1.0.0", &config.UpdateCheckConfig{
			Enabled: new(bool),
		})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())
		Expect(client.callCount).To(Equal(0))
	})

	It("fetches and notifies on fresh state", func() {
		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(ContainSubstring("1.0.0 -> 2.0.0"))
		Expect(client.callCount).To(Equal(1))

		// Verify state was saved
		state := updatecheck.LoadState(statePath())
		Expect(state.LatestVersion).To(Equal("v2.0.0"))
		Expect(state.NotifiedVersion).To(Equal("v2.0.0"))
	})

	It("uses cache when fresh", func() {
		// Pre-populate state
		state := &updatecheck.State{
			LastChecked:   now.Add(-1 * time.Hour), // 1h ago, within 24h
			LatestVersion: "v2.0.0",
		}
		Expect(updatecheck.SaveState(statePath(), state)).To(Succeed())

		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(ContainSubstring("1.0.0 -> 2.0.0"))
		Expect(client.callCount).To(Equal(0)) // No API call
	})

	It("refreshes stale cache", func() {
		// Pre-populate stale state
		state := &updatecheck.State{
			LastChecked:   now.Add(-25 * time.Hour), // 25h ago, stale
			LatestVersion: "v1.5.0",
		}
		Expect(updatecheck.SaveState(statePath(), state)).To(Succeed())

		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(ContainSubstring("1.0.0 -> 2.0.0"))
		Expect(client.callCount).To(Equal(1))
	})

	It("does not notify when already notified", func() {
		state := &updatecheck.State{
			LastChecked:     now.Add(-1 * time.Hour),
			LatestVersion:   "v2.0.0",
			NotifiedVersion: "v2.0.0",
		}
		Expect(updatecheck.SaveState(statePath(), state)).To(Succeed())

		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())
	})

	It("does not notify when current >= latest", func() {
		c := newChecker("3.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())
	})

	It("does not notify when current equals latest", func() {
		client.latestRelease = &github.Release{TagName: "v1.0.0"}
		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())
	})

	It("fails open on API error", func() {
		client.latestErr = errors.New("network error")
		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())
	})

	It("caches but does not notify when notify is disabled", func() {
		c := newChecker("1.0.0", &config.UpdateCheckConfig{
			Notify: new(bool),
		})
		msg := c.Check(context.Background())
		Expect(msg).To(BeEmpty())

		// But state was still updated with the latest version
		state := updatecheck.LoadState(statePath())
		Expect(state.LatestVersion).To(Equal("v2.0.0"))
	})

	It("respects custom check interval", func() {
		// Pre-populate state checked 2h ago
		state := &updatecheck.State{
			LastChecked:   now.Add(-2 * time.Hour),
			LatestVersion: "v1.5.0",
		}
		Expect(updatecheck.SaveState(statePath(), state)).To(Succeed())

		// With 1h interval, state is stale
		c := newChecker("1.0.0", &config.UpdateCheckConfig{
			CheckInterval: config.Duration(1 * time.Hour),
		})
		msg := c.Check(context.Background())
		Expect(msg).To(ContainSubstring("1.0.0 -> 2.0.0"))
		Expect(client.callCount).To(Equal(1))
	})

	It("notifies again for a newer version", func() {
		// Already notified for v1.5.0
		state := &updatecheck.State{
			LastChecked:     now.Add(-25 * time.Hour),
			LatestVersion:   "v1.5.0",
			NotifiedVersion: "v1.5.0",
		}
		Expect(updatecheck.SaveState(statePath(), state)).To(Succeed())

		// Now v2.0.0 is available
		c := newChecker("1.0.0", &config.UpdateCheckConfig{})
		msg := c.Check(context.Background())
		Expect(msg).To(ContainSubstring("1.0.0 -> 2.0.0"))
	})
})
