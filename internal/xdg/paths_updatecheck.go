package xdg

import "path/filepath"

// UpdateCheckStateFile returns StateDir()/update_check.json.
func UpdateCheckStateFile() string {
	return filepath.Join(StateDir(), "update_check.json")
}
