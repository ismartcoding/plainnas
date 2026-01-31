package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"ismartcoding/plainnas/internal/consts"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall PlainNAS (service, binaries, config, data)",
	Long: strings.TrimSpace(`Uninstall PlainNAS.

This command will best-effort:
- stop/disable the systemd service
- remove systemd unit file
- remove installed binaries (plainnas, plainnas-updater)
- remove /etc/plainnas (config, TLS, update key)
- remove /var/lib/plainnas (DB, indexes, cache)

It does NOT delete your media files under /mnt/usb*/.`),
	RunE: func(cmd *cobra.Command, args []string) error {
		if syscall.Getuid() != 0 {
			return fmt.Errorf("this command requires root privileges; run with sudo")
		}

		assumeYes, _ := cmd.Flags().GetBool("yes")
		keepConfig, _ := cmd.Flags().GetBool("keep-config")
		keepData, _ := cmd.Flags().GetBool("keep-data")
		serviceName, _ := cmd.Flags().GetString("service")
		binDir, _ := cmd.Flags().GetString("bin-dir")

		unitBase := strings.TrimSpace(serviceName)
		if unitBase == "" {
			unitBase = "plainnas"
		}
		unitBase = strings.TrimSuffix(unitBase, ".service")

		if !assumeYes {
			fmt.Fprintln(os.Stderr, "This will uninstall PlainNAS and may remove config/data.")
			fmt.Fprintln(os.Stderr, "- config: /etc/plainnas (unless --keep-config)")
			fmt.Fprintf(os.Stderr, "- data:   %s (unless --keep-data)\n", consts.DATA_DIR)
			fmt.Fprintln(os.Stderr, "- service/unit: systemd plainnas.service")
			fmt.Fprintln(os.Stderr, "- binaries: <bin-dir>/plainnas and <bin-dir>/plainnas-updater")
			confirm, err := readLineFromTTY(fmt.Sprintf("Type '%s' to confirm: ", unitBase))
			if err != nil {
				return err
			}
			if strings.TrimSpace(confirm) != unitBase {
				return fmt.Errorf("aborted")
			}
		}

		// 1) systemd service operations (best-effort)
		_ = trySystemctl("stop", unitBase)
		_ = trySystemctl("disable", unitBase)

		// 2) remove unit files
		unitCandidates := []string{
			filepath.Join("/etc/systemd/system", unitBase+".service"),
			filepath.Join("/lib/systemd/system", unitBase+".service"),
			filepath.Join("/usr/lib/systemd/system", unitBase+".service"),
		}
		for _, p := range unitCandidates {
			if err := removeFileIfExists(p); err != nil {
				return err
			}
		}

		_ = trySystemctl("daemon-reload")
		_ = trySystemctl("reset-failed", unitBase)

		// 3) remove binaries
		if strings.TrimSpace(binDir) == "" {
			binDir = "/usr/local/bin"
		}
		if err := removeFileIfExists(filepath.Join(binDir, "plainnas")); err != nil {
			return err
		}
		if err := removeFileIfExists(filepath.Join(binDir, "plainnas-updater")); err != nil {
			return err
		}

		// 4) remove config/data
		if !keepConfig {
			if err := removeAllIfExists("/etc/plainnas"); err != nil {
				return err
			}
		}
		if !keepData {
			if err := removeAllIfExists(consts.DATA_DIR); err != nil {
				return err
			}
		}

		fmt.Println("uninstall complete")
		return nil
	},
}

func init() {
	uninstallCmd.Flags().BoolP("yes", "y", false, "assume yes; do not prompt")
	uninstallCmd.Flags().Bool("keep-config", false, "keep /etc/plainnas")
	uninstallCmd.Flags().Bool("keep-data", false, "keep runtime data dir (default: /var/lib/plainnas)")
	uninstallCmd.Flags().String("service", "plainnas", "systemd service name")
	uninstallCmd.Flags().String("bin-dir", "/usr/local/bin", "directory containing installed binaries")
}

func readLineFromTTY(prompt string) (string, error) {
	in := os.Stdin
	out := os.Stderr
	fd := int(in.Fd())
	var tty *os.File

	if !term.IsTerminal(fd) {
		f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return "", fmt.Errorf("no TTY available for confirmation prompt")
		}
		tty = f
		in = f
		out = f
	}
	if tty != nil {
		defer tty.Close()
	}

	fmt.Fprint(out, prompt)
	r := bufio.NewReader(in)
	s, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func removeAllIfExists(path string) error {
	if err := os.RemoveAll(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func trySystemctl(args ...string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}

	c := exec.Command("systemctl", args...)
	out, err := c.CombinedOutput()
	if err != nil {
		// Best-effort: ignore common "unit not found" failures.
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "not loaded") || strings.Contains(msg, "could not be found") || strings.Contains(msg, "not-found") {
			return nil
		}
		return fmt.Errorf("systemctl %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}
