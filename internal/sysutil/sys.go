package sysutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Logger is the minimal logging interface used by this package.
type Logger interface {
	Printf(format string, v ...interface{})
}

// RunOptions configures RunCommand behaviour.
type RunOptions struct {
	Timeout time.Duration
	Dir     string
	Env     []string
	Stdout  io.Writer
	Stderr  io.Writer
	Logger  Logger
}

// RunResult captures command execution details.
type RunResult struct {
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	TimedOut bool
}

// RunCommand executes name with args using the provided options.
func RunCommand(opts RunOptions, name string, args ...string) (*RunResult, error) {
	if name == "" {
		return nil, errors.New("sysutil: command name is required")
	}
	cmdLine := commandString(name, args)
	start := time.Now()
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutWriter := io.Writer(&stdoutBuf)
	if opts.Stdout != nil {
		stdoutWriter = io.MultiWriter(opts.Stdout, &stdoutBuf)
	}
	stderrWriter := io.Writer(&stderrBuf)
	if opts.Stderr != nil {
		stderrWriter = io.MultiWriter(opts.Stderr, &stderrBuf)
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if opts.Logger != nil {
		opts.Logger.Printf("running command: %s", cmdLine)
	}
	err := cmd.Run()
	result := &RunResult{
		Command:  cmdLine,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: time.Since(start),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if opts.Logger != nil {
		opts.Logger.Printf("command finished (exit=%d, duration=%s)", result.ExitCode, result.Duration)
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		return result, fmt.Errorf("command timed out after %s: %s", opts.Timeout, cmdLine)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return result, fmt.Errorf("command failed with exit code %d: %s", exitErr.ExitCode(), cmdLine)
		}
		return result, fmt.Errorf("command failed: %w", err)
	}
	return result, nil
}

func commandString(name string, args []string) string {
	quoted := make([]string, 0, 1+len(args))
	quoted = append(quoted, shellQuote(name))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
