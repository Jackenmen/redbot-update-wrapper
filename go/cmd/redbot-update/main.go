package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cog-creators/redbot-update-wrapper/go/internal/osutils"
	"github.com/cog-creators/redbot-update-wrapper/go/internal/virtualenv"
)

const (
	envVarNamePrefix   = "REDBOT_UPDATE_WRAPPER_"
	LogDebugEnvVarName = envVarNamePrefix + "LOG_DEBUG"
	LogFileEnvVarName  = envVarNamePrefix + "LOG_FILE"
)

type pidLogValue struct {
	pid int
}

func (v pidLogValue) LogValue() slog.Value {
	if v.pid != 0 {
		return slog.IntValue(v.pid)
	}
	v.pid = os.Getpid()
	return slog.IntValue(v.pid)
}

type ProcessRunner struct {
	waiter     chan error
	currentCmd *exec.Cmd
}

func NewProcessRunner() *ProcessRunner {
	waiter := make(chan error)
	return &ProcessRunner{waiter: waiter}
}

func (r *ProcessRunner) Waiter() <-chan error {
	return r.waiter
}

func (r *ProcessRunner) Start(pythonExe string) error {
	args := append([]string{"-m", "redbot._update"}, os.Args[1:]...)
	cmd := exec.Command(pythonExe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	slog.Debug("Starting Python process", "args", args)
	if err := cmd.Start(); err != nil {
		slog.Debug("Failed to start Python process", "args", args, "error", err)
		return err
	}
	r.currentCmd = cmd

	go func() {
		exitCode := cmd.Wait()
		r.waiter <- exitCode
	}()

	return nil
}

func duplicateExe(exe string, venv virtualenv.VirtualEnv) string {
	// `.tmp` suffix should prevent the file from being executable on Windows,
	// while the chmod call should prevent it on Unix
	newLocation := filepath.Join(venv.GetBase(), DefaultProgramName+".tmp")
	log := slog.With("new_location", newLocation)

	log.Debug("Moving executable to new location")
	if err := os.Rename(exe, newLocation); err != nil {
		log.Debug("Failed to move executable to new location", "error", err)

		fmt.Printf(
			"%v\n\nFailed to move %v to %v. Another redbot-update process may already be running.",
			err, exe, newLocation,
		)
		os.Exit(1)
	}

	log.Debug("Copying executable")
	if copyErr := osutils.CopyFile(newLocation, exe); copyErr != nil {
		log.Debug("Failed to copy executable", "error", copyErr)

		if renameErr := os.Rename(newLocation, exe); renameErr != nil {
			log.Debug("Failed to revert executable move", "error", renameErr)

			err := fmt.Errorf("%w\n%w\nFailed to revert move", copyErr, renameErr)
			fmt.Printf(
				"%v\n\nFailed to copy %v to %v. Virtual environment is now broken"+
					" as we could not revert the earlier move of redbot-update's executable.",
				err, newLocation, exe,
			)
		} else {
			log.Debug("Reverted executable move", "error", renameErr)

			fmt.Printf("%v\n\nFailed to copy %v to %v.", copyErr, newLocation, exe)
		}

		os.Exit(1)
	}

	log.Debug("Making executable non-executable")
	if err := osutils.RemovePermissions(newLocation, 0111); err != nil {
		log.Debug("Failed to make executable non-executable", "error", err)

		fmt.Printf("%v\n\nFailed to make executable at %v non-executable.", err, newLocation)
		os.Exit(1)
	}

	return newLocation
}

func main() {
	handlerOptions := &slog.HandlerOptions{}
	if os.Getenv(LogDebugEnvVarName) == "1" {
		handlerOptions.Level = slog.LevelDebug
	}
	handlers := []slog.Handler{slog.NewTextHandler(os.Stderr, handlerOptions)}

	if filename := os.Getenv(LogFileEnvVarName); filename != "" {
		logFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		defer logFile.Close()
		handlers = append(handlers, slog.NewTextHandler(logFile, handlerOptions))
	}
	logger := slog.New(slog.NewMultiHandler(handlers...))
	slog.SetDefault(logger)

	slog.Debug("redbot-update wrapper started", "pid", pidLogValue{})

	exe, err := osutils.GetExecutableWithPreservedSymlinks(DefaultProgramName)
	if err != nil {
		slog.Debug("Failed to get executable", "error", err)
		panic(err)
	}
	slog.Debug("Found executable", "executable", exe)

	venv, err := virtualenv.GetVirtualEnv(exe)
	if err != nil {
		slog.Debug("Failed to get virtual environment", "error", err)
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	slog.Debug("Found virtual environment", "venv", venv)

	pythonExe, err := venv.GetPythonExecutable()
	if err != nil {
		slog.Debug("Failed to get Python executable", "error", err)
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	slog.Debug("Found Python executable", "python_executable", pythonExe)

	signalWaiter := make(chan os.Signal, 1)
	signal.Notify(signalWaiter, os.Interrupt, syscall.SIGTERM)

	isReg, err := osutils.IsRegular(exe)
	if err != nil {
		slog.Debug("Failed to stat executable", "error", err)
		fmt.Printf("Failed to stat executable:\n%v\n", err)
		os.Exit(1)
	}
	if isReg {
		isLink, err := osutils.IsSymlink(exe)
		if err != nil {
			slog.Debug("Failed to lstat executable", "error", err)
			fmt.Printf("Failed to lstat executable:\n%v\n", err)
			os.Exit(1)
		}
		if !isLink {
			// Since this is non-atomic, this has to be done after `signal.Notify()` call
			// to minimize the chance of us getting rid of our executable forever
			duplicateExe(exe, venv)
			// Technically, this duplicated exe never gets deleted but a Go binary is
			// just a few megabytes and Windows would not let you remove it anyway.
		}
	}

	runner := NewProcessRunner()
	if err := runner.Start(pythonExe); err != nil {
		fmt.Printf("Failed to start the process:\n%v\n", err)
		os.Exit(1)
	}
	processWaiter := runner.Waiter()

	for {
		select {
		case err := <-processWaiter:
			if err != nil {
				if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
					code := exitError.ExitCode()
					slog.Debug("Python process exited", "exit_code", code)
					os.Exit(code)
				} else {
					slog.Debug("Python process exited with an unexpected error", "error", err)
					fmt.Printf("Unexpected error occurred while running internal update command:\n%v\n", err)
					os.Exit(1)
				}
			}
			slog.Debug("Python process exited", "exit_code", 0)
			return
		case s := <-signalWaiter:
			slog.Debug("Received signal", "signal", s)
			// We don't need to do anything when receiving the signal
			// as it is automatically passed through to the child process
			// due to it being in the same process group.
		}
	}
}
