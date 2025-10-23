package logutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"velocloud-cloudinit-builder/internal/fsutil"
)

// NewOperationLogger creates a timestamped log file within baseDir/logs.
// The caller is responsible for closing the returned file handle.
func NewOperationLogger(baseDir, prefix string) (*log.Logger, *os.File, string, error) {
	if baseDir == "" {
		return nil, nil, "", fmt.Errorf("logutil: baseDir is required")
	}
	if prefix == "" {
		prefix = "log"
	}
	logDir := filepath.Join(baseDir, "logs")
	if err := fsutil.EnsureDir(logDir); err != nil {
		return nil, nil, "", err
	}
	filename := fmt.Sprintf("%s-%s.txt", prefix, time.Now().Format("20060102-150405"))
	fullPath := filepath.Join(logDir, filename)
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, nil, "", err
	}
	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("starting %s operation", prefix)
	return logger, f, fullPath, nil
}
