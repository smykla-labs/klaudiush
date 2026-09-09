package validators

import (
	"io"
	"os"

	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

// ReadCapped reads at most limit bytes of a regular file. The path comes from
// the command being validated, so an endless character device such as
// /dev/zero must not be read to the end, and a body far larger than any commit
// message or pull request description carries nothing worth matching. A
// missing, irregular or unreadable file reports false rather than an error:
// reading it is a best effort, not a gate.
func ReadCapped(log logger.Logger, path string, limit int64) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		log.Debug("Skipping request body file", "path", path, "error", err)

		return "", false
	}

	//nolint:gosec // path comes from the tool invocation klaudiush is validating
	file, err := os.Open(path)
	if err != nil {
		log.Debug("Cannot open request body file", "path", path, "error", err)

		return "", false
	}

	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		log.Debug("Cannot read request body file", "path", path, "error", err)

		return "", false
	}

	return string(content), true
}
