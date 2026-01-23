package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	service := flag.String("service", "plainnas", "systemd service name")
	binaryPath := flag.String("binary", "/usr/local/bin/plainnas", "path to current binary")
	newPath := flag.String("new", "", "path to prepared new binary (required)")
	oldPath := flag.String("old", "", "path to backup old binary")
	healthURL := flag.String("health", "http://127.0.0.1:8080/health_check", "health check URL")
	timeout := flag.Duration("timeout", 60*time.Second, "health check timeout")
	interval := flag.Duration("interval", 500*time.Millisecond, "health check interval")
	flag.Parse()

	if strings.TrimSpace(*newPath) == "" {
		exitErr(errors.New("--new is required"))
	}
	if strings.TrimSpace(*oldPath) == "" {
		*oldPath = defaultOldPath(*binaryPath)
	}

	if err := validateSameDir(*binaryPath, *newPath, *oldPath); err != nil {
		exitErr(err)
	}

	if _, err := os.Stat(*newPath); err != nil {
		exitErr(fmt.Errorf("new binary not found: %w", err))
	}

	if err := run("systemctl", "stop", *service); err != nil {
		exitErr(err)
	}

	// backup current
	_ = os.Remove(*oldPath)
	if err := os.Rename(*binaryPath, *oldPath); err != nil {
		// try to start service back
		_ = run("systemctl", "start", *service)
		exitErr(fmt.Errorf("backup failed: %w", err))
	}

	// promote new
	if err := os.Rename(*newPath, *binaryPath); err != nil {
		// rollback rename
		_ = os.Rename(*oldPath, *binaryPath)
		_ = run("systemctl", "start", *service)
		exitErr(fmt.Errorf("promote failed: %w", err))
	}

	if err := run("systemctl", "start", *service); err != nil {
		rollback(*service, *binaryPath, *oldPath)
		exitErr(err)
	}

	if err := waitHealthy(*healthURL, *timeout, *interval); err != nil {
		rollback(*service, *binaryPath, *oldPath)
		exitErr(err)
	}
}

func rollback(service, binaryPath, oldPath string) {
	_ = run("systemctl", "stop", service)
	_ = os.Remove(binaryPath)
	_ = os.Rename(oldPath, binaryPath)
	_ = run("systemctl", "start", service)
}

func waitHealthy(url string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("health check failed (timeout): %s", url)
		}
		res, err := client.Get(url)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(interval)
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultOldPath(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(binaryPath)
	return filepath.Join(dir, base+".old")
}

func validateSameDir(paths ...string) error {
	var dir string
	for _, p := range paths {
		d := filepath.Dir(p)
		if dir == "" {
			dir = d
			continue
		}
		if d != dir {
			return fmt.Errorf("paths must be on same filesystem/dir for atomic rename: %v", paths)
		}
	}
	return nil
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "[plainnas-updater]", err.Error())
	os.Exit(1)
}
