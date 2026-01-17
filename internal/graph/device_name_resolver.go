package graph

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ismartcoding/plainnas/internal/pkg/cmd"
)

func runCmd(name string, arg ...string) (string, error) {
	out, err := cmd.Exec(name, arg...)
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			return s, fmt.Errorf("%w", err)
		}
		return s, fmt.Errorf("%w: %s", err, s)
	}
	return s, nil
}

func sanitizeHostname(input string) string {
	v := strings.TrimSpace(input)
	v = strings.ToLower(v)
	v = strings.ReplaceAll(v, "_", "-")
	v = strings.ReplaceAll(v, " ", "-")
	v = strings.ReplaceAll(v, ".", "-")

	// allow only [a-z0-9-]
	var b strings.Builder
	b.Grow(len(v))
	prevDash := false
	for i := 0; i < len(v); i++ {
		ch := v[i]
		isAlphaNum := (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
		if isAlphaNum {
			b.WriteByte(ch)
			prevDash = false
			continue
		}
		if ch == '-' {
			if prevDash {
				continue
			}
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := b.String()
	out = strings.Trim(out, "-")
	if len(out) > 63 {
		out = out[:63]
		out = strings.Trim(out, "-")
	}
	return out
}

func setLinuxHostname(hostname string) error {
	if strings.TrimSpace(hostname) == "" {
		return errors.New("device_name_invalid")
	}
	if os.Geteuid() != 0 {
		return errors.New("device_name_requires_admin")
	}

	// Prefer hostnamectl when available.
	if _, err := exec.LookPath("hostnamectl"); err == nil {
		if _, err := runCmd("hostnamectl", "set-hostname", hostname); err == nil {
			return nil
		} else {
			return fmt.Errorf("device_name_apply_failed: hostnamectl failed: %v", err)
		}
	}

	// Fallback: write /etc/hostname and set runtime hostname.
	if err := os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0o644); err != nil {
		return fmt.Errorf("device_name_apply_failed: write /etc/hostname failed: %v", err)
	}
	if _, err := exec.LookPath("hostname"); err == nil {
		if _, err := runCmd("hostname", hostname); err != nil {
			return fmt.Errorf("device_name_apply_failed: hostname command failed: %v", err)
		}
	}
	return nil
}

func restartAvahiDaemon() error {
	// Service control only; PlainNAS does not implement mDNS itself.
	if os.Geteuid() != 0 {
		return errors.New("device_name_requires_admin")
	}
	// If Avahi isn't installed, report a stable error key.
	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		if _, statErr := os.Stat("/usr/sbin/avahi-daemon"); statErr != nil {
			return errors.New("device_name_avahi_not_installed")
		}
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		_, rerr := runCmd("systemctl", "restart", "avahi-daemon")
		if rerr != nil {
			// Common on minimal installs: systemd exists but avahi-daemon package isn't installed.
			if strings.Contains(rerr.Error(), "Unit avahi-daemon.service not found") {
				return errors.New("device_name_avahi_not_installed")
			}
			return fmt.Errorf("device_name_apply_failed: avahi restart failed: %v", rerr)
		}
		return nil
	}
	// Non-systemd environments: try service(8) if available.
	if _, err := exec.LookPath("service"); err == nil {
		_, rerr := runCmd("service", "avahi-daemon", "restart")
		if rerr != nil {
			return fmt.Errorf("device_name_apply_failed: avahi restart failed: %v", rerr)
		}
		return nil
	}
	return fmt.Errorf("device_name_apply_failed: could not restart avahi-daemon (systemctl/service not found)")
}
