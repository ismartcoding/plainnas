//go:build linux

package storage

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/pkg/log"
)

// RunAutoMountWatcher listens to Linux udev block events and triggers a remount
// reconciliation (EnsureMountedUSBVolumes). This is event-driven (no polling)
// and debounced to collapse bursts during hotplug.
func RunAutoMountWatcher(ctx context.Context) {
	go func() {
		if _, err := exec.LookPath("udevadm"); err != nil {
			log.Errorf("storage hotplug: udevadm not found: %v", err)
			return
		}

		cmd := exec.CommandContext(ctx, "udevadm", "monitor", "--udev", "--subsystem-match=block", "--property")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Errorf("storage hotplug: stdout pipe failed: %v", err)
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Errorf("storage hotplug: stderr pipe failed: %v", err)
			return
		}

		if err := cmd.Start(); err != nil {
			log.Errorf("storage hotplug: start udevadm failed: %v", err)
			return
		}

		// Drain stderr in background so udevadm can't block on a full pipe.
		go func() {
			s := bufio.NewScanner(stderr)
			for s.Scan() {
				// best-effort; we don't log every line to avoid noise
			}
		}()

		trigger := make(chan struct{}, 1)
		go debounceEnsure(ctx, trigger)

		scanner := bufio.NewScanner(stdout)
		props := map[string]string{}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				if shouldTriggerHotplug(props) {
					select {
					case trigger <- struct{}{}:
					default:
					}
				}
				props = map[string]string{}
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				props[k] = v
			}
		}

		_ = cmd.Wait()
	}()
}

func debounceEnsure(ctx context.Context, trigger <-chan struct{}) {
	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-trigger:
			if timer == nil {
				timer = time.NewTimer(700 * time.Millisecond)
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(700 * time.Millisecond)
		case <-func() <-chan time.Time {
			if timer == nil {
				return make(chan time.Time)
			}
			return timer.C
		}():
			timer = nil
			enCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := EnsureMountedUSBVolumes(enCtx)
			cancel()
			if err != nil {
				log.Errorf("storage hotplug: ensure mount failed: %v", err)
			}
		}
	}
}

func shouldTriggerHotplug(props map[string]string) bool {
	if len(props) == 0 {
		return false
	}
	if ss := props["SUBSYSTEM"]; ss != "" && ss != "block" {
		return false
	}
	action := props["ACTION"]
	switch action {
	case "add", "remove", "change":
		// ok
	default:
		return false
	}
	devType := props["DEVTYPE"]
	if devType != "" && devType != "disk" && devType != "partition" {
		return false
	}
	return true
}
