//go:build windows

package virtualenv

import "path"

func getPythonExecutablePath(venvDir string) string {
	return path.Join(venvDir, "Scripts", "python.exe")
}
