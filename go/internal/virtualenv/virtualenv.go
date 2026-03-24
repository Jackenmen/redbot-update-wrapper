package virtualenv

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type VirtualEnvSource int

const (
	SourceEnvVar VirtualEnvSource = iota
	SourceExeDir
)

var sourceName = map[VirtualEnvSource]string{
	SourceEnvVar: "EnvVar",
	SourceExeDir: "ExeDir",
}

type VirtualEnv struct {
	base   string
	source VirtualEnvSource
}

func (venv VirtualEnv) GetBase() string {
	return venv.base
}

func (venv VirtualEnv) GetSource() VirtualEnvSource {
	return venv.source
}

func (venv VirtualEnv) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("base", venv.base),
		slog.String("source", sourceName[venv.source]),
	)
}

func (venv VirtualEnv) GetPythonExecutable() (string, error) {
	p := getPythonExecutablePath(venv.base)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%w\n\nCould not find a Python executable at %v", err, p)
	}
	return p, nil
}

func (venv VirtualEnv) GetPyVenvConfigPath() string {
	return filepath.Join(venv.base, "pyvenv.cfg")
}

func (venv VirtualEnv) GetPyVenvConfig() (map[string]string, error) {
	cfgPath := venv.GetPyVenvConfigPath()
	file, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("%w\n\nCould not open %v file, is this not a venv?", err, cfgPath)
	}
	defer file.Close()

	pyvenvCfg := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if found {
			pyvenvCfg[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return pyvenvCfg, fmt.Errorf(
			"Unexpected error occurred while parsing the %v file:\n%w", cfgPath, err,
		)
	}

	return pyvenvCfg, nil
}

func GetVirtualEnv(exe string) (VirtualEnv, error) {
	virtualEnvVar := os.Getenv("VIRTUAL_ENV")
	venv := VirtualEnv{base: virtualEnvVar, source: SourceEnvVar}
	if virtualEnvVar != "" {
		return venv, nil
	}

	// assume that our executable (`redbot-update`) resides in venv's scripts directory
	venv.source = SourceExeDir
	scriptsDir := filepath.Dir(exe)
	unresolvedVenvDir := filepath.Dir(scriptsDir)
	venv.base = unresolvedVenvDir
	venvDir, err := filepath.EvalSymlinks(unresolvedVenvDir)
	if err != nil {
		return venv, fmt.Errorf(
			"Unexpected error occurred while trying to resolve symlinks for the venv dir (%v):\n%w",
			unresolvedVenvDir,
			err,
		)
	}
	venv.base = venvDir
	pyvenvCfgPath := venv.GetPyVenvConfigPath()

	file, err := os.Open(pyvenvCfgPath)
	if err != nil {
		return venv, fmt.Errorf(
			"%w\n\nCould not open %v file, is this not a venv?", err, pyvenvCfgPath,
		)
	}
	file.Close()

	return venv, nil
}
