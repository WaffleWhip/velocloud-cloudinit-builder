package fsutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// EnsureDir creates the directory when it does not already exist.
func EnsureDir(path string) error {
	if path == "" {
		return errors.New("fsutil: empty path for EnsureDir")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return nil
}

// PathExists reports whether the given path exists.
func PathExists(path string) (bool, error) {
	if path == "" {
		return false, errors.New("fsutil: empty path for PathExists")
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// RemoveIfExists removes the provided path if it exists. No error when missing.
func RemoveIfExists(path string) error {
	if path == "" {
		return errors.New("fsutil: empty path for RemoveIfExists")
	}
	err := os.RemoveAll(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SafeJoin joins the provided path elements, ensuring the final path stays rooted.
func SafeJoin(root string, elems ...string) (string, error) {
	if root == "" {
		return "", errors.New("fsutil: empty root for SafeJoin")
	}
	full := filepath.Join(append([]string{root}, elems...)...)
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return filepath.Clean(full), nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("fsutil: path escapes root")
	}
	return filepath.Clean(full), nil
}

// CopyStream writes src into destPath, creating parent directories as needed.
func CopyStream(destPath string, r io.Reader) error {
	if destPath == "" {
		return errors.New("fsutil: empty destPath for CopyStream")
	}
	if err := EnsureDir(filepath.Dir(destPath)); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// CopyFile duplicates the contents and permissions of src into dest.
func CopyFile(src, dest string) error {
	if src == "" || dest == "" {
		return errors.New("fsutil: CopyFile requires non-empty src and dest")
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("fsutil: stat source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("fsutil: source is not a regular file: %s", src)
	}
	if err := EnsureDir(filepath.Dir(dest)); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
