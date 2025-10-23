package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"velocloud-cloudinit-builder/internal/builder"
	"velocloud-cloudinit-builder/internal/deps"
	"velocloud-cloudinit-builder/internal/logutil"
	"velocloud-cloudinit-builder/internal/output"
	"velocloud-cloudinit-builder/internal/vmtest"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := stripGlobalFlags(os.Args[1:])
	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}

	if len(args) == 0 {
		return runInteractive(baseDir)
	}

	switch args[0] {
	case "build":
		return builder.Build(baseDir)
	case "test":
		return runTest(baseDir, args[1:])
	case "uninstall":
		return runUninstall(baseDir, args[1:])
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runInteractive(baseDir string) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println()
		fmt.Println("=== CloudInit Builder ===")
		fmt.Println("1) Build cloud-init ISO")
		fmt.Println("2) Jalankan VM test")
		fmt.Println("3) Uninstall & bersihkan")
		fmt.Println("4) Keluar")
		fmt.Print("Pilih menu [1-4]: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			if err := builder.Build(baseDir); err != nil {
				fmt.Fprintf(os.Stderr, "Gagal build ISO: %v\n", err)
				continue
			}
			if promptYesNo(reader, "Tes VM sekarang? [Y/n]: ") {
				vmPath := promptVMPath(reader)
				if err := vmtest.Run(baseDir, vmPath, nil); err != nil {
					fmt.Fprintf(os.Stderr, "Gagal menjalankan VM: %v\n", err)
				}
			}
		case "2":
			vmPath := promptVMPath(reader)
			if err := vmtest.Run(baseDir, vmPath, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Gagal menjalankan VM: %v\n", err)
			}
		case "3":
			if !promptYesNo(reader, "Uninstall akan menghapus semua file. Lanjut? [y/N]: ") {
				continue
			}
			if err := runUninstall(baseDir, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Uninstall gagal: %v\n", err)
			}
		case "4", "q", "Q", "exit", "keluar":
			fmt.Println("Keluar dari CloudInit Builder.")
			return nil
		default:
			fmt.Println("Pilihan tidak dikenali, silakan ulangi.")
		}
	}
}

func runTest(baseDir string, args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	vmPath := fs.String("vm", "", "Path to a portable VM executable (optional)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(os.Stdout)
			fs.Usage()
			return nil
		}
		return err
	}
	return vmtest.Run(baseDir, *vmPath, fs.Args())
}

func runUninstall(baseDir string, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	selfDelete := fs.Bool("self-delete", false, "Delete the executable after uninstall")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(os.Stdout)
			fs.Usage()
			return nil
		}
		return err
	}

	logger, logFile, logPath, err := logutil.NewOperationLogger(baseDir, "uninstall")
	if err != nil {
		return err
	}
	output.Printf("[*] Logging uninstall output to %s\n", relPath(baseDir, logPath))

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = filepath.Join(baseDir, "cloudinit-builder.exe")
	} else {
		binaryPath, _ = filepath.Abs(binaryPath)
	}

	output.Println("[*] Removing tools/, images/, runtime/, cache/, templates/")
	if err := deps.PerformUninstall(baseDir, *selfDelete, binaryPath, logger); err != nil {
		logFile.Close()
		return err
	}
	logger.Printf("closing log file prior to deleting logs directory")
	if err := logFile.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}

	logsPath := filepath.Join(baseDir, "logs")
	if err := os.RemoveAll(logsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove logs directory: %w", err)
	}
	if *selfDelete {
		output.Println("[*] Deleting binary after exit...")
	}
	output.Println("[+] Uninstall complete.")
	return nil
}

func relPath(baseDir, target string) string {
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return target
	}
	return rel
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  cloudinit-builder [-q|--quiet] build")
	fmt.Fprintln(w, "  cloudinit-builder [-q|--quiet] test [--vm <path-to-portable-vm>] [-- <vm-extra-args>]")
	fmt.Fprintln(w, "  cloudinit-builder [-q|--quiet] uninstall [--self-delete]")
}

func stripGlobalFlags(args []string) []string {
	if len(args) == 0 {
		return args
	}
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "-q", "--quiet":
			output.SetQuiet(true)
		default:
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func promptYesNo(reader *bufio.Reader, label string) bool {
	fmt.Print(label)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return true
	}
	resp = strings.ToLower(resp)
	return resp == "y" || resp == "yes"
}

func promptVMPath(reader *bufio.Reader) string {
	fmt.Print("Path VM portable (enter untuk gunakan QEMU bawaan): ")
	resp, _ := reader.ReadString('\n')
	return strings.TrimSpace(resp)
}
