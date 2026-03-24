//go:build windows

package virtualenv

import "path/filepath"

func getPythonExecutablePath(venvDir string) string {
	return filepath.Join(venvDir, "Scripts", "python.exe")
}
