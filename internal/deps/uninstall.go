package deps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"velocloud-cloudinit-builder/internal/fsutil"
	"velocloud-cloudinit-builder/internal/sysutil"
)

// PerformUninstall removes runtime assets and optionally deletes the executable.
func PerformUninstall(baseDir string, selfDelete bool, binaryPath string, logger sysutil.Logger) error {
	if logger != nil {
		logger.Printf("starting uninstall from %s", baseDir)
	}
	if err := killProcesses(baseDir, logger, "podman.exe", "qemu-system-x86_64.exe"); err != nil && logger != nil {
		logger.Printf("warning: failed to terminate some helper processes: %v", err)
	}

	podmanExe := filepath.Join(baseDir, "tools", "podman", "podman.exe")
	if exists, _ := fsutil.PathExists(podmanExe); exists {
		if err := RemovePodmanMachine(baseDir, podmanExe, logger); err != nil && logger != nil {
			logger.Printf("warning: failed to remove podman machine: %v", err)
		}
	}

	targets := []string{
		filepath.Join(baseDir, "tools"),
		filepath.Join(baseDir, "images"),
		filepath.Join(baseDir, "runtime"),
		filepath.Join(baseDir, "cache"),
		filepath.Join(baseDir, "templates"),
	}
	for _, path := range targets {
		if err := fsutil.RemoveIfExists(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		if logger != nil {
			logger.Printf("removed %s", path)
		}
	}

	if !selfDelete {
		return nil
	}

	if binaryPath == "" {
		return fmt.Errorf("cannot self-delete: binary path unknown")
	}
	scriptPath := filepath.Join(baseDir, fmt.Sprintf("cleanup-%d.bat", time.Now().Unix()))
	if err := scheduleSelfDelete(scriptPath, binaryPath, logger); err != nil {
		return err
	}
	return nil
}

func killProcesses(baseDir string, logger sysutil.Logger, processNames ...string) error {
	var aggregate error
	for _, name := range processNames {
		result, err := sysutil.RunCommand(sysutil.RunOptions{
			Timeout: 5 * time.Second,
			Dir:     baseDir,
			Logger:  logger,
		}, "taskkill", "/IM", name, "/T", "/F")
		if err != nil {
			if result != nil && result.ExitCode == 128 {
				continue
			}
			if logger != nil {
				logger.Printf("warning: failed to kill %s: %v", name, err)
			}
			aggregate = errors.Join(aggregate, fmt.Errorf("kill %s: %w", name, err))
		}
	}
	return aggregate
}

func scheduleSelfDelete(scriptPath, binaryPath string, logger sysutil.Logger) error {
	scriptContent := fmt.Sprintf(`@echo off
timeout /t 2 >nul
del "%s"
del "%%~f0"
`, binaryPath)
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("created self-delete script %s", scriptPath)
	}
	_, err := sysutil.RunCommand(sysutil.RunOptions{
		Timeout: 2 * time.Second,
	}, "cmd.exe", "/C", "start", "", scriptPath)
	if err != nil {
		return fmt.Errorf("launch cleanup script: %w", err)
	}
	if logger != nil {
		logger.Printf("scheduled self-delete via %s", scriptPath)
	}
	return nil
}
