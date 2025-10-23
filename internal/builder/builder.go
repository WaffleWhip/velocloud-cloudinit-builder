package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"velocloud-cloudinit-builder/internal/deps"
	"velocloud-cloudinit-builder/internal/fsutil"
	"velocloud-cloudinit-builder/internal/logutil"
	"velocloud-cloudinit-builder/internal/output"
	"velocloud-cloudinit-builder/internal/sysutil"
)

const (
	imageName         = "docker.io/library/debian:bookworm"
	podmanPullTimeout = 10 * time.Minute
	podmanRunTimeout  = 15 * time.Minute
	buildLogPrefix    = "build"
)

// Build orchestrates the ISO creation flow.
func Build(baseDir string) (err error) {
	logger, logFile, logPath, err := logutil.NewOperationLogger(baseDir, buildLogPrefix)
	if err != nil {
		return err
	}
	defer logFile.Close()

	output.Printf("[*] Logging build output to %s\n", pathRelative(baseDir, logPath))
	output.Println("[*] Checking dependencies...")
	if err := deps.EnsureBaseLayout(baseDir, logger); err != nil {
		return fmt.Errorf("ensure base layout: %w", err)
	}

	if err := deps.EnsureTemplates(baseDir, logger); err != nil {
		return fmt.Errorf("ensure templates: %w", err)
	}

	var podmanPath string
	var machineName string
	var podmanEnv []string

	defer func() {
		if podmanPath == "" || machineName == "" || len(podmanEnv) == 0 {
			return
		}
		if stopErr := deps.StopPodmanMachine(baseDir, podmanPath, machineName, podmanEnv, logFile, logger); stopErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to stop podman machine: %v\n", stopErr)
		} else if err == nil {
			output.Println("[*] Podman machine stopped.")
		}
	}()

	podmanPath, err = deps.EnsurePodman(baseDir, logger)
	if err != nil {
		return fmt.Errorf("ensure podman: %w", err)
	}
	output.Println("[*] Podman ready.")

	machineName, podmanEnv, err = deps.EnsurePodmanMachine(baseDir, podmanPath, logFile, logger)
	if err != nil {
		return fmt.Errorf("ensure podman machine: %w", err)
	}

	output.Println("[*] Pulling Debian image...")
	if err := runPodman(baseDir, podmanPath, machineName, podmanEnv, []string{"pull", imageName}, logFile, logger, podmanPullTimeout); err != nil {
		return fmt.Errorf("podman pull: %w", err)
	}

	output.Println("[*] Building cloud-init.iso with genisoimage...")
	if err := runPodmanRun(baseDir, podmanPath, machineName, podmanEnv, logFile, logger); err != nil {
		return fmt.Errorf("podman run: %w", err)
	}

	output.Println("[+] Done: images/cloud-init.iso created.")
	return nil
}

func runPodman(baseDir, podmanPath, machineName string, env []string, args []string, logFile *os.File, logger sysutil.Logger, timeout time.Duration) error {
	allArgs := append([]string{"--connection", machineName}, args...)
	_, err := sysutil.RunCommand(sysutil.RunOptions{
		Timeout: timeout,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logFile,
		Stderr:  logFile,
		Env:     env,
	}, podmanPath, allArgs...)
	return err
}

func runPodmanRun(baseDir, podmanPath, machineName string, env []string, logFile *os.File, logger sysutil.Logger) error {
	isoPath := filepath.Join(baseDir, "images", "cloud-init.iso")
	if err := os.Remove(isoPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	mountArg := fmt.Sprintf("%s:/work", filepath.Clean(baseDir))
	buildScript := strings.Join([]string{
		"set -euo pipefail",
		"apt-get update -qq",
		"apt-get install -y genisoimage",
		"genisoimage -output /work/images/cloud-init.iso -volid cidata -joliet -rock -graft-points user-data=/work/templates/user-data.txt meta-data=/work/templates/meta-data.txt",
	}, " && ")
	podmanArgs := []string{
		"run",
		"--rm",
		"-v", mountArg,
		"-w", "/work",
		imageName,
		"bash",
		"-c",
		buildScript,
	}
	if err := fsutil.EnsureDir(filepath.Dir(isoPath)); err != nil {
		return err
	}
	_, err := sysutil.RunCommand(sysutil.RunOptions{
		Timeout: podmanRunTimeout,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logFile,
		Stderr:  logFile,
		Env:     env,
	}, podmanPath, append([]string{"--connection", machineName}, podmanArgs...)...)
	return err
}

func pathRelative(baseDir, target string) string {
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return target
	}
	return rel
}
