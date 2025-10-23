package vmtest

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
	testLogPrefix   = "test"
	vmRunTimeout    = 30 * time.Minute
	isoRelativePath = "images/cloud-init.iso"
	qcowRelative    = "images/velocloud.qcow2"
)

// Run starts a VM with the generated ISO for validation. When vmPath is empty, a bundled QEMU build is used.
func Run(baseDir, vmPath string, passthroughArgs []string) error {
	logger, logFile, logPath, err := logutil.NewOperationLogger(baseDir, testLogPrefix)
	if err != nil {
		return err
	}
	defer logFile.Close()

	output.Printf("[*] Logging test output to %s\n", relPath(baseDir, logPath))

	var absVM string
	usingBundledQEMU := false
	if vmPath == "" {
		output.Println("[*] Preparing bundled QEMU runtime...")
		absVM, err = deps.EnsureQEMU(baseDir, logger)
		if err != nil {
			return fmt.Errorf("ensure qemu: %w", err)
		}
		usingBundledQEMU = true
	} else {
		absVM, err = filepath.Abs(vmPath)
		if err != nil {
			return fmt.Errorf("resolve vm path: %w", err)
		}
		if err := ensureFileExists(absVM, "VM executable"); err != nil {
			return err
		}
	}

	isoPath := filepath.Join(baseDir, filepath.FromSlash(isoRelativePath))
	if err := ensureFileExists(isoPath, "cloud-init ISO"); err != nil {
		return err
	}
	qcowPath := filepath.Join(baseDir, filepath.FromSlash(qcowRelative))
	if err := ensureFileExists(qcowPath, "velocloud qcow2 image"); err != nil {
		return err
	}

	tempDir := filepath.Join(baseDir, "runtime", "vm")
	if err := fsutil.EnsureDir(tempDir); err != nil {
		return fmt.Errorf("prepare vm runtime dir: %w", err)
	}
	cloneName := fmt.Sprintf("velocloud-%s.qcow2", time.Now().Format("20060102-150405"))
	clonePath := filepath.Join(tempDir, cloneName)
	output.Printf("[*] Cloning base qcow2 to %s\n", relPath(baseDir, clonePath))
	if err := fsutil.CopyFile(qcowPath, clonePath); err != nil {
		return fmt.Errorf("clone qcow2: %w", err)
	}
	defer func() {
		if err := fsutil.RemoveIfExists(clonePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete temp disk %s: %v\n", clonePath, err)
		} else {
			output.Printf("[*] Deleted temporary disk %s\n", relPath(baseDir, clonePath))
		}
	}()

	var args []string
	if usingBundledQEMU || looksLikeQEMU(absVM) {
		output.Println("[*] Launching QEMU with qcow2 + ISO...")
		args = defaultQEMUArgs(clonePath, isoPath)
	} else {
		output.Println("[*] Launching provided VM executable...")
		args = []string{"--disk", clonePath, "--cdrom", isoPath}
	}
	if len(passthroughArgs) > 0 {
		args = append(args, passthroughArgs...)
	}

	if _, err := sysutil.RunCommand(sysutil.RunOptions{
		Timeout: vmRunTimeout,
		Dir:     baseDir,
		Logger:  logger,
		Stdout:  logFile,
		Stderr:  logFile,
	}, absVM, args...); err != nil {
		return fmt.Errorf("vm execution failed: %w", err)
	}

	output.Println("[+] VM process exited normally.")
	return nil
}

func defaultQEMUArgs(diskPath, isoPath string) []string {
	return []string{
		"-name", "cloudinit-builder-test,process=cloudinit-builder-test",
		"-m", "4096",
		"-smp", "2",
		"-drive", fmt.Sprintf("if=virtio,format=qcow2,file=%s", diskPath),
		"-cdrom", isoPath,
		"-boot", "d",
		"-accel", defaultAccel(),
		"-netdev", "user,id=wan,ipv6=off",
		"-device", "virtio-net-pci,netdev=wan,mac=52:54:00:00:00:01",
		"-vga", "std",
		"-display", "sdl",
		"-serial", "stdio",
	}
}

func defaultAccel() string {
	if v := strings.TrimSpace(os.Getenv("CLOUDINIT_BUILDER_QEMU_ACCEL")); v != "" {
		return v
	}
	return "tcg"
}

func looksLikeQEMU(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasPrefix(base, "qemu-system") || strings.Contains(base, "qemu")
}

func ensureFileExists(path string, description string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found at %s", description, path)
		}
		return fmt.Errorf("stat %s: %w", description, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s points to a directory: %s", description, path)
	}
	return nil
}

func relPath(baseDir, target string) string {
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return target
	}
	return rel
}
