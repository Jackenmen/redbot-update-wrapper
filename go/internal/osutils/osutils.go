package osutils

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// We query the working directory at init, to use it later to search for the
// executable file
// errWd will be checked later, if we need to use initWd
var initWd, errWd = os.Getwd()

func GetExecutableWithPreservedSymlinks(defaultProgramName string) (string, error) {
	programName := ""
	if len(os.Args) > 0 {
		programName = os.Args[0]
	}
	if programName == "" {
		programName = defaultProgramName
	}

	if filepath.IsAbs(programName) {
		return filepath.Clean(programName), nil
	}

	if strings.ContainsRune(programName, os.PathSeparator) {
		if errWd != nil {
			return "", errWd
		}
		return filepath.Join(initWd, programName), nil
	}

	realExe, err := GetRealExecutable()
	if err != nil || realExe != "" {
		return realExe, err
	}

	for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		exe := ""
		if filepath.IsAbs(p) {
			exe = filepath.Join(p, programName)
		} else if errWd != nil {
			return "", errWd
		} else {
			exe = filepath.Join(initWd, p, programName)
		}
		if IsExecutable(exe) {
			return exe, nil
		}
	}
	return "", os.ErrNotExist
}

func IsRegular(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}

func IsSymlink(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

func CopyFile(src, dst string) error {
	fsrc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fsrc.Close()

	fi, err := fsrc.Stat()
	if err != nil {
		return err
	}
	dstMode := fi.Mode()

	fdst, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fdst.Close()

	if _, err := io.Copy(fdst, fsrc); err != nil {
		return err
	}
	fdst.Close()

	return os.Chmod(dst, dstMode)
}

func addPermissions(path string, perms os.FileMode) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	return os.Chmod(path, fi.Mode()|perms)
}

func removePermissions(path string, perms os.FileMode) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	return os.Chmod(path, fi.Mode()&^perms)
}
