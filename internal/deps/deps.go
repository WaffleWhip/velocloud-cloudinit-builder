package deps

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"velocloud-cloudinit-builder/internal/fsutil"
	"velocloud-cloudinit-builder/internal/sysutil"
)

const (
	podmanVersionTag = "v5.1.0"
	podmanZipURL     = "https://github.com/containers/podman/releases/download/v5.1.0/podman-remote-release-windows_amd64.zip"
	podmanZipName    = "podman-remote-release-windows_amd64.zip"
)

var baseDirs = []string{
	"tools",
	"tools/podman",
	"tools/qemu",
	"templates",
	"images",
	"runtime",
	"runtime/podman",
	"runtime/podman/tmp",
	"runtime/podman/config",
	"runtime/podman/run",
	"runtime/podman/home",
	"cache",
	"logs",
}

// EnsureBaseLayout guarantees the directory skeleton exists.
func EnsureBaseLayout(baseDir string, logger sysutil.Logger) error {
	for _, rel := range baseDirs {
		target := filepath.Join(baseDir, filepath.FromSlash(rel))
		if err := fsutil.EnsureDir(target); err != nil {
			return err
		}
	}
	if logger != nil {
		logger.Printf("base layout verified at %s", baseDir)
	}
	return nil
}

// EnsureTemplates creates default template files if they do not already exist.
func EnsureTemplates(baseDir string, logger sysutil.Logger) error {
	templateDir := filepath.Join(baseDir, "templates")
	userData := filepath.Join(templateDir, "user-data.txt")
	metaData := filepath.Join(templateDir, "meta-data.txt")

	if created, err := ensureFileWithContent(userData, defaultUserData(), logger); err != nil {
		return err
	} else if created && logger != nil {
		logger.Printf("created default user-data template at %s", userData)
	}
	if created, err := ensureFileWithContent(metaData, defaultMetaData(), logger); err != nil {
		return err
	} else if created && logger != nil {
		logger.Printf("created default meta-data template at %s", metaData)
	}
	return nil
}

func ensureFileWithContent(path string, content string, logger sysutil.Logger) (bool, error) {
	exists, err := fsutil.PathExists(path)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := fsutil.CopyStream(path, strings.NewReader(content)); err != nil {
		return false, err
	}
	return true, nil
}

// EnsurePodman makes sure podman.exe is available locally and returns its path.
func EnsurePodman(baseDir string, logger sysutil.Logger) (string, error) {
	podmanDir := filepath.Join(baseDir, "tools", "podman")
	podmanExe := filepath.Join(podmanDir, "podman.exe")

	exists, err := fsutil.PathExists(podmanExe)
	if err != nil {
		return "", err
	}
	if exists {
		if err := copySupportBinaries(podmanDir); err != nil {
			return "", err
		}
		if logger != nil {
			logger.Printf("podman already present at %s", podmanExe)
		}
		return podmanExe, nil
	}

	if logger != nil {
		logger.Printf("podman not found, downloading portable release %s", podmanVersionTag)
	}
	cacheDir := filepath.Join(baseDir, "cache")
	if err := fsutil.EnsureDir(cacheDir); err != nil {
		return "", err
	}
	zipPath := filepath.Join(cacheDir, podmanZipName)
	if err := downloadFile(podmanZipURL, zipPath, logger); err != nil {
		return "", err
	}

	if logger != nil {
		logger.Printf("extracting podman archive %s", zipPath)
	}
	if err := fsutil.RemoveIfExists(podmanDir); err != nil {
		return "", err
	}
	if err := fsutil.EnsureDir(podmanDir); err != nil {
		return "", err
	}
	if err := extractZip(zipPath, podmanDir); err != nil {
		return "", err
	}
	if err := placePodmanExecutable(podmanDir); err != nil {
		return "", err
	}
	if err := copySupportBinaries(podmanDir); err != nil {
		return "", err
	}
	if logger != nil {
		logger.Printf("podman ready at %s", podmanExe)
	}
	return podmanExe, nil
}

func downloadFile(url, dest string, logger sysutil.Logger) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	tmpDest := dest + ".tmp"
	out, err := os.Create(tmpDest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpDest, dest); err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("downloaded %s (%d bytes)", dest, resp.ContentLength)
	}
	return nil
}

func extractZip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath := filepath.Join(dest, file.Name)
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(dest)) {
			return fmt.Errorf("unsafe path in archive: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := fsutil.EnsureDir(targetPath); err != nil {
				return err
			}
			continue
		}
		if err := fsutil.EnsureDir(filepath.Dir(targetPath)); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			src.Close()
			return err
		}
		out.Close()
		src.Close()
	}
	return nil
}

func placePodmanExecutable(podmanDir string) error {
	return copyBinaryIfNeeded(podmanDir, "podman.exe")
}

func copySupportBinaries(podmanDir string) error {
	for _, name := range []string{"win-sshproxy.exe", "gvproxy.exe"} {
		if err := copyBinaryIfNeeded(podmanDir, name); err != nil {
			return err
		}
	}
	return nil
}

func copyBinaryIfNeeded(rootDir, binaryName string) error {
	target := filepath.Join(rootDir, binaryName)
	exists, err := fsutil.PathExists(target)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	var found string
	err = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), binaryName) {
			found = path
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if found == "" {
		return fmt.Errorf("%s not found after extraction", binaryName)
	}
	in, err := os.Open(found)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func defaultUserData() string {
	return strings.Join([]string{
		"#cloud-config",
		"hostname: vce",
		"password: Velocloud123",
		"chpasswd: {expire: False}",
		"ssh_pwauth: True",
	}, "\n") + "\n"
}

func defaultMetaData() string {
	return strings.Join([]string{
		"instance-id: vce",
		"local-hostname: vce",
	}, "\n") + "\n"
}
