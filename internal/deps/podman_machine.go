package deps

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"velocloud-cloudinit-builder/internal/fsutil"
	"velocloud-cloudinit-builder/internal/sysutil"
)

const (
	podmanMachineName   = "cloudinit-builder"
	machineInitTimeout  = 10 * time.Minute
	machineStartTimeout = 3 * time.Minute
)

// EnsurePodmanMachine makes sure a dedicated podman machine exists and is running.
// It returns the machine name and the environment variables to be used for podman commands.
func EnsurePodmanMachine(baseDir, podmanPath string, logWriter io.Writer, logger sysutil.Logger) (string, []string, error) {
	env, err := podmanEnv(baseDir)
	if err != nil {
		return "", nil, err
	}

	if err := ensureMachineExists(baseDir, podmanPath, env, logWriter, logger); err != nil {
		return "", nil, err
	}
	if err := ensureMachineRunning(baseDir, podmanPath, env, logWriter, logger); err != nil {
		return "", nil, err
	}
	if err := ensureDefaultConnection(baseDir, podmanPath, env, logWriter, logger); err != nil {
		return "", nil, err
	}
	return podmanMachineName, env, nil
}

func podmanEnv(baseDir string) ([]string, error) {
	configDir := filepath.Join(baseDir, "runtime", "podman", "config")
	tmpDir := filepath.Join(baseDir, "runtime", "podman", "tmp")
	runDir := filepath.Join(baseDir, "runtime", "podman", "run")
	homeDir := filepath.Join(baseDir, "runtime", "podman", "home")
	for _, dir := range []string{configDir, tmpDir, runDir, homeDir} {
		if err := fsutil.EnsureDir(dir); err != nil {
			return nil, err
		}
	}
	return []string{
		"XDG_CONFIG_HOME=" + configDir,
		"XDG_RUNTIME_DIR=" + runDir,
		"PODMAN_CONFIG=" + filepath.Join(configDir, "containers.conf"),
		"PODMAN_TMPDIR=" + tmpDir,
		"TMPDIR=" + tmpDir,
		"HOME=" + homeDir,
	}, nil
}

func ensureMachineExists(baseDir, podmanPath string, env []string, logWriter io.Writer, logger sysutil.Logger) error {
	opts := sysutil.RunOptions{
		Timeout: machineStartTimeout,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logWriter,
		Stderr:  logWriter,
		Env:     env,
	}
	result, err := sysutil.RunCommand(opts, podmanPath, "machine", "inspect", podmanMachineName)
	if err == nil {
		return nil
	}
	if !machineMissing(err, result) {
		return fmt.Errorf("check podman machine: %w", err)
	}

	if logger != nil {
		logger.Printf("initializing podman machine %s", podmanMachineName)
	}
	opts.Timeout = machineInitTimeout
	if cleanupErr := cleanupMachineConnection(baseDir, podmanPath, env, logWriter, logger); cleanupErr != nil && logger != nil {
		logger.Printf("warning: failed to clean stale connection: %v", cleanupErr)
	}
	if _, err = sysutil.RunCommand(opts, podmanPath, "machine", "init", podmanMachineName, "--now"); err != nil {
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "connection") && strings.Contains(errLower, "already exists") {
			if cleanupErr := cleanupMachineConnection(baseDir, podmanPath, env, logWriter, logger); cleanupErr != nil && logger != nil {
				logger.Printf("warning: failed to clean stale connection: %v", cleanupErr)
			}
			if _, retryErr := sysutil.RunCommand(opts, podmanPath, "machine", "init", podmanMachineName, "--now"); retryErr != nil {
				return fmt.Errorf("podman machine init after cleanup: %w", retryErr)
			}
			return nil
		}
		return fmt.Errorf("podman machine init: %w", err)
	}
	return nil
}

func ensureMachineRunning(baseDir, podmanPath string, env []string, logWriter io.Writer, logger sysutil.Logger) error {
	state, err := machineState(baseDir, podmanPath, env, logWriter, logger)
	if err != nil {
		return err
	}
	if strings.TrimSpace(state) == "running" {
		return nil
	}
	if logger != nil {
		logger.Printf("starting podman machine %s", podmanMachineName)
	}
	opts := sysutil.RunOptions{
		Timeout: machineStartTimeout,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logWriter,
		Stderr:  logWriter,
		Env:     env,
	}
	if _, err := sysutil.RunCommand(opts, podmanPath, "machine", "start", podmanMachineName); err != nil {
		return fmt.Errorf("podman machine start: %w", err)
	}
	return nil
}

func ensureDefaultConnection(baseDir, podmanPath string, env []string, logWriter io.Writer, logger sysutil.Logger) error {
	opts := sysutil.RunOptions{
		Timeout: 30 * time.Second,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logWriter,
		Stderr:  logWriter,
		Env:     env,
	}
	if _, err := sysutil.RunCommand(opts, podmanPath, "system", "connection", "default", podmanMachineName); err != nil {
		if !strings.Contains(err.Error(), "already default") {
			return fmt.Errorf("set podman connection default: %w", err)
		}
	}
	return nil
}

func machineState(baseDir, podmanPath string, env []string, logWriter io.Writer, logger sysutil.Logger) (string, error) {
	opts := sysutil.RunOptions{
		Timeout: 30 * time.Second,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logWriter,
		Stderr:  logWriter,
		Env:     env,
	}
	result, err := sysutil.RunCommand(opts, podmanPath, "machine", "inspect", podmanMachineName, "--format", "{{.State}}")
	if err != nil {
		return "", fmt.Errorf("podman machine inspect: %w", err)
	}
	return strings.TrimSpace(result.Stdout), nil
}

func machineMissing(err error, result *sysutil.RunResult) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such vm") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist") {
		return true
	}
	if result != nil {
		stdout := strings.ToLower(result.Stdout)
		stderr := strings.ToLower(result.Stderr)
		if strings.Contains(stdout, "does not exist") ||
			strings.Contains(stderr, "does not exist") ||
			strings.Contains(stdout, "no such vm") ||
			strings.Contains(stderr, "no such vm") {
			return true
		}
	}
	return false
}

func cleanupMachineConnection(baseDir, podmanPath string, env []string, logWriter io.Writer, logger sysutil.Logger) error {
	names := []string{podmanMachineName, podmanMachineName + "-root"}
	for _, name := range names {
		opts := sysutil.RunOptions{
			Timeout: 30 * time.Second,
			Dir:     baseDir,
			Logger:  logger,
			Stdout:  logWriter,
			Stderr:  logWriter,
			Env:     env,
		}
		result, err := sysutil.RunCommand(opts, podmanPath, "system", "connection", "rm", name)
		if err != nil {
			if machineMissing(err, result) {
				continue
			}
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "no such connection") || strings.Contains(errLower, "not found") {
				continue
			}
			return err
		}
	}
	return nil
}

// StopPodmanMachine stops the running podman machine if it exists.
func StopPodmanMachine(baseDir, podmanPath, machineName string, env []string, logWriter io.Writer, logger sysutil.Logger) error {
	if machineName == "" {
		machineName = podmanMachineName
	}
	opts := sysutil.RunOptions{
		Timeout: machineStartTimeout,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logWriter,
		Stderr:  logWriter,
		Env:     env,
	}
	result, err := sysutil.RunCommand(opts, podmanPath, "machine", "stop", machineName)
	if err != nil {
		if machineMissing(err, result) || strings.Contains(strings.ToLower(err.Error()), "already stopped") {
			return nil
		}
		return fmt.Errorf("podman machine stop: %w", err)
	}
	return nil
}

// RemovePodmanMachine stops and removes the managed podman machine.
func RemovePodmanMachine(baseDir, podmanPath string, logger sysutil.Logger) error {
	env, err := podmanEnv(baseDir)
	if err != nil {
		return err
	}
	if err := StopPodmanMachine(baseDir, podmanPath, podmanMachineName, env, nil, logger); err != nil {
		return err
	}
	opts := sysutil.RunOptions{
		Timeout: machineStartTimeout,
		Dir:     baseDir,
		Logger:  logger,
		Env:     env,
	}
	result, err := sysutil.RunCommand(opts, podmanPath, "machine", "rm", "-f", podmanMachineName)
	if err != nil {
		if machineMissing(err, result) {
			return nil
		}
		return fmt.Errorf("podman machine rm: %w", err)
	}
	if cleanupErr := cleanupMachineConnection(baseDir, podmanPath, env, nil, logger); cleanupErr != nil && logger != nil {
		logger.Printf("warning: failed to clean connection after removal: %v", cleanupErr)
	}
	return nil
}
