package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/cog-creators/redbot-update-wrapper/go/internal/osutils"
	"github.com/cog-creators/redbot-update-wrapper/go/internal/virtualenv"
)

const DefaultProgramName = "redbot-update"

func main() {
	exe, err := osutils.GetExecutableWithPreservedSymlinks(DefaultProgramName)
	if err != nil {
		panic(err)
	}
	scriptsDir := path.Dir(exe)
	venvDir := path.Dir(scriptsDir)
	pyvenvCfgPath := path.Join(venvDir, "pyvenv.cfg")

	file, err := os.Open(pyvenvCfgPath)
	if err != nil {
		fmt.Printf("%v\n\nCould not open %v file, is this not a venv?\n", err, pyvenvCfgPath)
		os.Exit(1)
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
		fmt.Printf("Unexpected error occurred while parsing the %v file:\n%v\n", err)
		os.Exit(1)
	}
	file.Close()

	pythonExe, err := virtualenv.GetPythonExecutable(venvDir)
	if err != nil {
		fmt.Printf("%v\n\nCould not find a Python executable for venv at %v\n", err, venvDir)
		os.Exit(1)
	}

	args := append([]string{"-m", "redbot._update"}, os.Args[1:]...)
	cmd := exec.Command(pythonExe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exitError.ExitCode())
		} else {
			fmt.Printf("Unexpected error occurred while running internal update command:\n%v\n", err)
			os.Exit(1)
		}
	}
}
