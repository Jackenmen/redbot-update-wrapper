//go:build unix

package osutils

import "os"

func IsExecutable(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := stat.Mode()
	if !mode.IsRegular() {
		return false
	}
	if (mode & 0111) == 0 {
		return false
	}
	return true
}

func GetRealExecutable() (string, error) {
	return "", nil
}
