package updatecheck

import (
	"fmt"
	"strings"
)

// FormatNotification builds the human-readable update notification message.
func FormatNotification(currentVersion, latestVersion string) string {
	current := strings.TrimPrefix(currentVersion, "v")
	latest := strings.TrimPrefix(latestVersion, "v")

	return fmt.Sprintf(
		"Update available: klaudiush %s -> %s. Run 'klaudiush update' to install.",
		current, latest,
	)
}
