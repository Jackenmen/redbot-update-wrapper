//go:build windows

package osutils

import (
	"os"
	"strings"
)

func IsExecutable(path string) bool {
	stat, err := os.Lstat(path)
	if err != nil {
		return false
	}
	mode := stat.Mode()
	if !mode.IsRegular() {
		return false
	}
	if len(path) < 4 {
		return false
	}
	return strings.EqualFold(path[len(path)-4:], ".exe")
}

func GetRealExecutable() (string, error) {
	return os.Executable()
}
