package update

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunUpdaterViaSystemdRun launches the updater as a transient unit so it survives `systemctl stop plainnas`.
func RunUpdaterViaSystemdRun(updaterPath string, plan Plan) error {
	args := []string{
		"--unit=plainnas-updater",
		"--collect",
		"--property=Type=oneshot",
		updaterPath,
		"--service", plan.ServiceName,
		"--binary", plan.BinaryPath,
		"--new", plan.NewPath,
		"--old", plan.OldPath,
		"--health", plan.HealthURL,
	}
	cmd := exec.Command("systemd-run", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemd-run failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
