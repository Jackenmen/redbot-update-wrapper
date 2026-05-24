package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cog-creators/redbot-update-wrapper/go/internal/logutils"
)

type RequestInput struct {
	RequestType         string             `json:"request_type"`
	RequestNewPythonExe string             `json:"request_new_python_exe"`
	RequestNewStartArgs []string           `json:"request_new_start_args"`
	RequestSetEnvVars   map[string]*string `json:"request_set_env_vars"`
}

type RequestOutput interface {
	SetRequestType(string)
}

type ExecRequestInput struct {
	*RequestInput
}

type ExecRequestOutput struct {
	RequestType string `json:"request_type"`
}

func (o *ExecRequestOutput) SetRequestType(v string) {
	o.RequestType = v
}

type SpawnProcessRequestInput struct {
	*RequestInput
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type SpawnProcessRequestOutput struct {
	RequestType string `json:"request_type"`
	ExitCode    int    `json:"exit_code"`
	Exited      bool   `json:"exited"`
	Pid         int    `json:"pid"`
	// these don't seem likely to be useful, especially since they're system specific
	// but might as well include them...
	Sys        any           `json:"sys"`
	SysUsage   any           `json:"sys_usage"`
	SystemTime time.Duration `json:"system_time"`
	UserTime   time.Duration `json:"user_time"`
}

func (o *SpawnProcessRequestOutput) SetRequestType(v string) {
	o.RequestType = v
}

type ProcessRunner struct {
	currentCmd     *exec.Cmd
	wrapperVersion string
	wrapperExe     string
	runnerDir      string
	pythonExe      string
	startArgs      []string
}

func NewProcessRunner(wrapperVersion, wrapperExe, pythonExe string) *ProcessRunner {
	return &ProcessRunner{
		wrapperVersion: wrapperVersion,
		wrapperExe:     wrapperExe,
		pythonExe:      pythonExe,
		startArgs:      append([]string{"-m", "redbot._update"}, os.Args[1:]...),
	}
}

func (r *ProcessRunner) cleanupRunnerDir() error {
	if r.runnerDir != "" {
		return os.RemoveAll(r.runnerDir)
	}
	r.runnerDir = ""
	return nil
}

func (r *ProcessRunner) Close() error {
	return r.cleanupRunnerDir()
}

func (r *ProcessRunner) handleRequest() error {
	slog.Debug("Received a runner request")

	filename := filepath.Join(r.runnerDir, "request_input.json")
	log := slog.With("input_file", filename)
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Debug("Failed to open request input file")
		return err
	}

	var input RequestInput
	if err = json.Unmarshal(data, &input); err != nil {
		log.Debug("Failed to parse request input file")
		return err
	}
	slog.Debug("Parsed request input", "request", logutils.NewStructLogValue(&input))

	for k, v := range input.RequestSetEnvVars {
		if v == nil {
			err = os.Unsetenv(k)
		} else {
			err = os.Setenv(k, *v)
		}
		if err != nil {
			log.Debug("Failed to set env var", "env_var_name", k, "env_var_value", v)
			return err
		}
	}

	var output RequestOutput
	switch input.RequestType {
	case "exec":
		output, err = r.handleExecRequest(data)
	case "spawn_command":
		output, err = r.handleSpawnCommandRequest(data)
	default:
		err = fmt.Errorf("Received invalid request type: %s", input.RequestType)
	}
	if err != nil {
		return err
	}
	output.SetRequestType(input.RequestType)

	if err := r.makeNewRunnerDir(); err != nil {
		return err
	}
	if err := r.writeRequestOutput(output); err != nil {
		return err
	}

	r.pythonExe = input.RequestNewPythonExe
	r.startArgs = input.RequestNewStartArgs
	return r.Run()
}

func (r *ProcessRunner) handleExecRequest(data []byte) (*ExecRequestOutput, error) {
	return &ExecRequestOutput{}, nil
}

func (r *ProcessRunner) handleSpawnCommandRequest(data []byte) (*SpawnProcessRequestOutput, error) {
	var input SpawnProcessRequestInput
	if err := json.Unmarshal(data, &input); err != nil {
		slog.Debug("Failed to parse spawn command request input")
		return nil, err
	}
	slog.Debug("Parsed spawn command request input", "request", input)

	cmd := exec.Command(input.Command, input.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if input.Env != nil {
		env := []string{}
		for k, v := range input.Env {
			env = append(env, fmt.Sprintf("%v=%v", k, v))
		}
		cmd.Env = env
	}

	slog.Debug("Running command", "command", input.Command, "args", input.Args)
	if err := cmd.Run(); err != nil {
		slog.Debug("Spawned command returned an error", "err", err)
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return nil, err
		}
	}
	output := &SpawnProcessRequestOutput{
		ExitCode:   cmd.ProcessState.ExitCode(),
		Exited:     cmd.ProcessState.Exited(),
		Pid:        cmd.ProcessState.Pid(),
		Sys:        cmd.ProcessState.Sys(),
		SysUsage:   cmd.ProcessState.SysUsage(),
		SystemTime: cmd.ProcessState.SystemTime(),
		UserTime:   cmd.ProcessState.UserTime(),
	}
	return output, nil
}

func (r *ProcessRunner) writeRequestOutput(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Debug("Failed to serialize request output", "output", v)
		return err
	}
	filename := filepath.Join(r.runnerDir, "request_output.json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		slog.Debug("Failed to save request output", "output_file", filename)
		return err
	}
	return nil
}

func (r *ProcessRunner) makeNewRunnerDir() error {
	if err := r.cleanupRunnerDir(); err != nil {
		return err
	}
	slog.Debug("Making new temporary runner dir")
	runnerDir, err := os.MkdirTemp("", "redbot-update-*")
	if err != nil {
		return err
	}
	r.runnerDir = runnerDir
	slog.Debug("Created new temporary runner dir", "runner_dir", r.runnerDir)
	return nil
}

func (r *ProcessRunner) Start() error {
	if r.runnerDir == "" {
		if err := r.makeNewRunnerDir(); err != nil {
			return err
		}
	}
	log := slog.With("runner_dir", r.runnerDir)

	cmd := exec.Command(r.pythonExe, r.startArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"REDBOT_UPDATE_RUNNER_WRAPPER_VERSION="+r.wrapperVersion,
		"REDBOT_UPDATE_RUNNER_WRAPPER_EXE="+r.wrapperExe,
		"REDBOT_UPDATE_RUNNER_DIR="+r.runnerDir,
	)
	log.Debug("Starting Python process", "args", r.startArgs)
	if err := cmd.Start(); err != nil {
		log.Debug("Failed to start Python process", "args", r.startArgs, "error", err)
		return err
	}
	r.currentCmd = cmd

	return nil
}

func (r *ProcessRunner) Wait() error {
	cmdErr := r.currentCmd.Wait()
	if exitError, ok := errors.AsType[*exec.ExitError](cmdErr); ok {
		exitCode := exitError.ExitCode()
		if exitCode == HandleRequestExitCode {
			return r.handleRequest()
		}
	}
	if cmdErr == nil {
		return r.Close()
	}
	return cmdErr
}

func (r *ProcessRunner) Run() error {
	if err := r.Start(); err != nil {
		return err
	}
	return r.Wait()
}
