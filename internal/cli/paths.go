package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHome expands a leading "~" or "~/" to the user's home directory.
// Git Rodolfo expands paths itself before writing them anywhere (PRD
// §12.2: "Rutas absolutas"), rather than depending on the shell.
func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// displayPath collapses a leading home directory back to "~" for display —
// the inverse of expandHome, used to show scanned key paths the way a user
// would type them (RF-40).
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	return path
}
