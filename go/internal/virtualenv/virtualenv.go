package virtualenv

import "os"

func GetPythonExecutable(venvDir string) (string, error) {
	p := getPythonExecutablePath(venvDir)
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}
