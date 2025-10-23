package deps

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"velocloud-cloudinit-builder/internal/fsutil"
	"velocloud-cloudinit-builder/internal/sysutil"
)

var errStopWalk = errors.New("qemu-stop-walk")

const (
	qemuVersionTag = "20240822"
	qemuZipName    = "qemu-w64-portable-20240822.zip"
	qemuZipURL     = "https://github.com/dirkarnez/qemu-portable/releases/download/20240822/qemu-w64-portable-20240822.zip"
	qemuExeName    = "qemu-system-x86_64.exe"
)

// EnsureQEMU ensures that a portable QEMU build is available locally and returns the absolute executable path.
func EnsureQEMU(baseDir string, logger sysutil.Logger) (string, error) {
	qemuDir := filepath.Join(baseDir, "tools", "qemu")
	if err := fsutil.EnsureDir(qemuDir); err != nil {
		return "", err
	}

	if exe, err := findQEMUExecutable(qemuDir); err == nil {
		if logger != nil {
			logger.Printf("qemu already present at %s", exe)
		}
		return exe, nil
	}

	if logger != nil {
		logger.Printf("qemu not found, downloading portable release %s", qemuVersionTag)
	}
	cacheDir := filepath.Join(baseDir, "cache")
	if err := fsutil.EnsureDir(cacheDir); err != nil {
		return "", err
	}
	zipPath := filepath.Join(cacheDir, qemuZipName)
	if err := downloadFile(qemuZipURL, zipPath, logger); err != nil {
		return "", err
	}

	if logger != nil {
		logger.Printf("extracting qemu archive %s", zipPath)
	}
	if err := fsutil.RemoveIfExists(qemuDir); err != nil {
		return "", err
	}
	if err := fsutil.EnsureDir(qemuDir); err != nil {
		return "", err
	}
	if err := extractZip(zipPath, qemuDir); err != nil {
		return "", err
	}

	exe, err := findQEMUExecutable(qemuDir)
	if err != nil {
		return "", err
	}
	if logger != nil {
		logger.Printf("qemu ready at %s", exe)
	}
	return exe, nil
}

func findQEMUExecutable(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if strings.EqualFold(d.Name(), qemuExeName) {
			found = path
			return errStopWalk
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopWalk) {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found inside %s", qemuExeName, root)
	}
	return found, nil
}
