//go:build unix

package virtualenv

import "path/filepath"

func getPythonExecutablePath(venvDir string) string {
	return filepath.Join(venvDir, "bin", "python")
}
