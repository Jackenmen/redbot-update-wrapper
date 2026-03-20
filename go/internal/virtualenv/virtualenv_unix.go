//go:build unix

package virtualenv

import "path"

func getPythonExecutablePath(venvDir string) string {
	return path.Join(venvDir, "bin", "python")
}
