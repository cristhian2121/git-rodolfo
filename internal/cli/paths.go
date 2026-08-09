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
