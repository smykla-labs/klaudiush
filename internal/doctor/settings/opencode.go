package settings

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cockroachdb/errors"

	"github.com/smykla-skalski/klaudiush/internal/xdg"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

//go:embed templates/opencode_plugin.ts
var openCodePluginTemplate embed.FS

const (
	openCodePluginTemplatePath = "templates/opencode_plugin.ts"

	// openCodePluginRelPath is the plugin location under the opencode config
	// directory. opencode loads every file in this directory at startup.
	openCodePluginRelPath = ".config/opencode/plugin/klaudiush.ts"

	// openCodePluginPermissions keeps the generated plugin readable by the
	// opencode runtime, which runs as the same user but re-reads the file.
	openCodePluginPermissions = 0o640
)

// ErrPluginNotFound is returned when the opencode bridge plugin is absent.
var ErrPluginNotFound = errors.New("opencode plugin not found")

// DefaultOpenCodePluginPath returns the default bridge plugin location.
func DefaultOpenCodePluginPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, openCodePluginRelPath)
}

// OpenCodePluginParser inspects the generated opencode bridge plugin.
type OpenCodePluginParser struct {
	pluginPath string
}

// NewOpenCodePluginParser creates a parser for the given plugin path.
func NewOpenCodePluginParser(path string) *OpenCodePluginParser {
	return &OpenCodePluginParser{pluginPath: path}
}

// Read returns the plugin source.
func (p *OpenCodePluginParser) Read() (string, error) {
	resolvedPath, err := resolveSettingsPath(p.pluginPath)
	if err != nil {
		return "", err
	}

	data, err := readFileAt(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrPluginNotFound
		}

		return "", errors.Wrap(err, "failed to read opencode plugin")
	}

	return string(data), nil
}

// IsDispatcherRegistered reports whether the plugin invokes this dispatcher.
func (p *OpenCodePluginParser) IsDispatcherRegistered(dispatcherPath string) (bool, error) {
	source, err := p.Read()
	if err != nil {
		if errors.Is(err, ErrPluginNotFound) {
			return false, nil
		}

		return false, err
	}

	return strings.Contains(source, dispatcherPath), nil
}

// HasEventHook reports whether the plugin forwards the given opencode event.
func (p *OpenCodePluginParser) HasEventHook(eventName, dispatcherPath string) (bool, error) {
	source, err := p.Read()
	if err != nil {
		if errors.Is(err, ErrPluginNotFound) {
			return false, nil
		}

		return false, err
	}

	if !strings.Contains(source, dispatcherPath) {
		return false, nil
	}

	return pluginRegistersEvent(source, eventName), nil
}

// pluginRegistersEvent reports whether the plugin source actually subscribes to
// an event, as opposed to merely mentioning its name.
//
// A plain substring search is not enough: every forwarded event appears as an
// argument to invoke(), so any such search reports an event as configured even
// when nothing is listening for it. The plugin subscribes in exactly two ways,
// and this checks for both:
//
//   - a hook key, `"tool.execute.before":`, optionally behind opencode's
//     `experimental.` prefix for hooks that are still unstable
//   - a case label on the shared event bus, `case "session.idle":`
func pluginRegistersEvent(source, eventName string) bool {
	for _, form := range []string{
		`"` + eventName + `":`,
		`"experimental.` + eventName + `":`,
		`case "` + eventName + `":`,
	} {
		if strings.Contains(source, form) {
			return true
		}
	}

	return false
}

// InstallOpenCodeDispatcher renders the bridge plugin for the given binary.
// It reports whether the plugin was already installed and up to date, matching
// the return convention of the other provider installers.
func InstallOpenCodeDispatcher(pluginPath, binaryPath string) (bool, error) {
	rendered, err := RenderOpenCodePlugin(binaryPath)
	if err != nil {
		return false, err
	}

	resolvedPath, err := resolveSettingsPath(pluginPath)
	if err != nil {
		return false, err
	}

	if existing, readErr := readFileAt(resolvedPath); readErr == nil {
		if bytes.Equal(existing, rendered) {
			return true, nil
		}
	}

	if err := writeOpenCodePlugin(resolvedPath, rendered); err != nil {
		return false, err
	}

	return false, nil
}

// RenderOpenCodePlugin renders the bridge plugin source for a binary path.
func RenderOpenCodePlugin(binaryPath string) ([]byte, error) {
	raw, err := openCodePluginTemplate.ReadFile(openCodePluginTemplatePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read embedded opencode plugin")
	}

	tmpl, err := template.New("opencode_plugin").Parse(string(raw))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse opencode plugin template")
	}

	var buf bytes.Buffer

	data := struct {
		BinaryPath string
		TimeoutMs  int
	}{
		BinaryPath: binaryPath,
		TimeoutMs:  DefaultCommandHookTimeout * millisecondsPerSecond,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.Wrap(err, "failed to render opencode plugin")
	}

	return buf.Bytes(), nil
}

// OpenCodeEventNames returns the opencode events the bridge plugin forwards.
func OpenCodeEventNames() []string {
	return hook.OpenCodeEventNames()
}

// readFileAt reads an operator-configured path. Isolated so the gosec
// suppression stays attached to the single call it applies to.
func readFileAt(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // path comes from operator config
}

func writeOpenCodePlugin(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), defaultDirPermissions); err != nil {
		return errors.Wrap(err, "failed to create opencode plugin directory")
	}

	if err := xdg.EnsureDir(filepath.Dir(path)); err != nil {
		return errors.Wrap(err, "failed to prepare opencode plugin directory")
	}

	if err := os.WriteFile(path, data, openCodePluginPermissions); err != nil {
		return errors.Wrap(err, "failed to write opencode plugin")
	}

	return nil
}
