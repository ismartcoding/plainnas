package samba

import (
	"bytes"
	"fmt"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/pkg/cmd"
	"ismartcoding/plainnas/internal/pkg/systemctl"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	unixUser    = "nas"
	smbConfPath = "/etc/samba/smb.conf"
)

type ServiceStatus struct {
	Active  bool
	Enabled bool
	Name    string
}

func GetServiceStatus() ServiceStatus {
	name := detectSystemdServiceName()
	if name == "" {
		return ServiceStatus{}
	}
	s := systemctl.GetStatus(name)
	return ServiceStatus{Active: s.Active, Enabled: s.Enabled, Name: name}
}

func Apply(settings db.SambaSettings, password string) error {
	// The caller stores desired settings first; we use the passed value as source of truth.
	// (Do not read back from DB here, to avoid surprising races.)
	desired := settings

	if desired.Enabled {
		if len(desired.Shares) == 0 {
			return fmt.Errorf("no shares configured")
		}
		for _, sh := range desired.Shares {
			if strings.TrimSpace(sh.SharePath) == "" {
				return fmt.Errorf("share path is empty")
			}
			if err := os.MkdirAll(sh.SharePath, 0o755); err != nil {
				return err
			}
			if st, err := os.Stat(sh.SharePath); err != nil {
				return err
			} else if !st.IsDir() {
				return fmt.Errorf("share path is not a directory")
			}
		}
	}

	if err := ensureUnixUser(); err != nil {
		return err
	}

	if err := writeSmbConf(desired); err != nil {
		return err
	}

	if desired.Enabled && strings.TrimSpace(password) != "" {
		if err := ensureSambaPassword(password); err != nil {
			return err
		}
		desired.HasPassword = true
		_ = db.StoreSambaSettings(desired)
	}

	service := detectSystemdServiceName()
	if service == "" {
		return fmt.Errorf("systemd service for samba not found")
	}

	if !desired.Enabled {
		_, _ = systemctl.Start(service, false)
		_, _ = systemctl.Enable(service, false)
		return nil
	}

	_, _ = systemctl.Enable(service, true)
	_, err := systemctl.Start(service, true)
	return err
}

func detectSystemdServiceName() string {
	candidates := []string{"smbd", "samba", "smb"}
	for _, name := range candidates {
		if isSystemdUnitLoaded(name) {
			return name
		}
	}
	return ""
}

func isSystemdUnitLoaded(serviceName string) bool {
	out, err := cmd.Run(fmt.Sprintf("sudo systemctl show %s --property=LoadState", serviceName))
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "LoadState=loaded")
}

func ensureUnixUser() error {
	if err := exec.Command("id", "-u", unixUser).Run(); err == nil {
		return nil
	}

	if hasCmd("useradd") {
		// -M: no home, -r: system user (best-effort), -s: no login
		_, err := cmd.Run(fmt.Sprintf("sudo useradd -M -r -s /usr/sbin/nologin %s", unixUser))
		return err
	}
	if hasCmd("adduser") {
		// Alpine: adduser -D creates user with no password, -H no home
		_, err := cmd.Run(fmt.Sprintf("sudo adduser -D -H -s /sbin/nologin %s", unixUser))
		return err
	}
	return fmt.Errorf("cannot create unix user %q: useradd/adduser not found", unixUser)
}

func ensureSambaPassword(password string) error {
	pw := strings.TrimSpace(password)
	if pw == "" {
		return nil
	}
	if !hasCmd("smbpasswd") {
		return fmt.Errorf("smbpasswd not found")
	}

	// -s: read password from stdin
	// -a: add user if missing
	c := exec.Command("sudo", "smbpasswd", "-a", "-s", unixUser)
	c.Stdin = bytes.NewBufferString(pw + "\n" + pw + "\n")
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func SetUserPassword(password string) error {
	if err := ensureUnixUser(); err != nil {
		return err
	}
	return ensureSambaPassword(password)
}

func writeSmbConf(s db.SambaSettings) error {
	if err := os.MkdirAll(filepath.Dir(smbConfPath), 0o755); err != nil {
		return err
	}

	// macOS Finder works best with Samba's fruit module. Some distros/minimal installs
	// may not ship these modules; only enable them when present, otherwise Samba fails to start.
	fruitAvailable := vfsModuleAvailable("fruit")
	catiaAvailable := vfsModuleAvailable("catia")
	streamsXattrAvailable := vfsModuleAvailable("streams_xattr")

	var b bytes.Buffer
	b.WriteString("# Managed by PlainNAS.\n")
	b.WriteString("# Manual edits may be overwritten from the Web Settings page.\n\n")
	b.WriteString("[global]\n")
	b.WriteString("  workgroup = WORKGROUP\n")
	b.WriteString("  server role = standalone server\n")
	b.WriteString("  map to guest = Bad User\n")
	b.WriteString("  load printers = no\n")
	b.WriteString("  disable spoolss = yes\n")
	b.WriteString("  log file = /var/log/samba/log.%m\n")
	b.WriteString("  max log size = 1000\n")
	b.WriteString("  server min protocol = SMB2\n")
	if fruitAvailable {
		// Enables Apple SMB2 extensions (AAPL). This improves Finder interoperability.
		b.WriteString("  fruit:aapl = yes\n")
	}
	b.WriteString("\n")

	if s.Enabled {
		used := map[string]struct{}{}
		for i, sh := range s.Shares {
			name := sanitizeShareName(sh.Name)
			if name == "" {
				name = "share"
			}
			// Ensure uniqueness.
			base := name
			for n := 2; ; n++ {
				if _, ok := used[strings.ToLower(name)]; !ok {
					break
				}
				name = fmt.Sprintf("%s-%d", base, n)
			}
			used[strings.ToLower(name)] = struct{}{}

			b.WriteString(fmt.Sprintf("[%s]\n", name))
			b.WriteString(fmt.Sprintf("  path = %s\n", sh.SharePath))
			b.WriteString("  browseable = yes\n")

			// macOS compatibility: enable fruit + (optional) streams_xattr when possible.
			// For filesystems without xattr support (common for some USB/external drives),
			// fall back to AppleDouble sidecar files instead of xattrs.
			if fruitAvailable {
				xattrOK := supportsXattrs(sh.SharePath)
				vfs := make([]string, 0, 3)
				if catiaAvailable {
					vfs = append(vfs, "catia")
				}
				vfs = append(vfs, "fruit")
				if xattrOK && streamsXattrAvailable {
					vfs = append(vfs, "streams_xattr")
					b.WriteString("  ea support = yes\n")
					b.WriteString("  store dos attributes = yes\n")
					b.WriteString("  fruit:metadata = stream\n")
					b.WriteString("  fruit:resource = stream\n")
				} else {
					b.WriteString("  fruit:metadata = netatalk\n")
					b.WriteString("  fruit:resource = file\n")
				}

				// Common Finder-friendly settings.
				b.WriteString("  fruit:posix_rename = yes\n")
				b.WriteString("  fruit:zero_file_id = yes\n")
				b.WriteString("  fruit:delete_empty_adfiles = yes\n")
				b.WriteString("  vfs objects = " + strings.Join(vfs, " ") + "\n")
			}

			b.WriteString("  force user = " + unixUser + "\n")
			b.WriteString("  create mask = 0664\n")
			b.WriteString("  directory mask = 0775\n")
			_ = i

			switch sh.Auth {
			case db.SambaShareAuthGuest:
				b.WriteString("  guest ok = yes\n")
				b.WriteString("  guest only = yes\n")
			case db.SambaShareAuthPassword:
				b.WriteString("  guest ok = no\n")
				b.WriteString("  valid users = " + unixUser + "\n")
			default:
				b.WriteString("  guest ok = yes\n")
				b.WriteString("  guest only = yes\n")
			}

			if sh.ReadOnly {
				b.WriteString("  read only = yes\n")
			} else {
				b.WriteString("  read only = no\n")
			}
			b.WriteString("\n")
		}
	}

	if err := os.WriteFile(smbConfPath, b.Bytes(), 0o644); err != nil {
		return err
	}

	// Validate if possible (best-effort)
	if hasCmd("testparm") {
		_, _ = cmd.Run("testparm -s " + smbConfPath)
	}
	return nil
}

func sanitizeShareName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.Trim(s, "[]")
	if s == "" {
		return ""
	}
	// Replace unsupported characters with underscore.
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		case r == ' ':
			return '_'
		default:
			return '_'
		}
	}, s)
	out = strings.Trim(out, "-_. ")
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func supportsXattrs(dir string) bool {
	// Best-effort probe: if xattrs aren't supported, streams_xattr will not work.
	// We prefer probing a file inside the directory (more representative) and we
	// are conservative: if we cannot verify, return false.
	attr := "user.plainnas_xattr_probe"
	value := []byte("1")

	tmp, err := os.CreateTemp(dir, ".plainnas-xattr-*")
	if err == nil {
		path := tmp.Name()
		_ = tmp.Close()
		defer func() { _ = os.Remove(path) }()
		if err := unix.Setxattr(path, attr, value, 0); err != nil {
			if err == unix.ENOTSUP || err == unix.EOPNOTSUPP || err == unix.ENOSYS {
				return false
			}
			// If we cannot set xattrs due to permissions/policy, treat as unsupported.
			if err == unix.EPERM || err == unix.EACCES {
				return false
			}
			return false
		}
		_ = unix.Removexattr(path, attr)
		return true
	}

	// Fallback: probe the directory itself.
	if err := unix.Setxattr(dir, attr, value, 0); err != nil {
		if err == unix.ENOTSUP || err == unix.EOPNOTSUPP || err == unix.ENOSYS {
			return false
		}
		if err == unix.EPERM || err == unix.EACCES {
			return false
		}
		return false
	}
	_ = unix.Removexattr(dir, attr)
	return true
}

func vfsModuleAvailable(name string) bool {
	candidates := []string{"vfs_" + name + ".so", name + ".so"}

	// Prefer asking smbd for MODULESDIR when available.
	if hasCmd("smbd") {
		out, err := exec.Command("smbd", "-b").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "MODULESDIR:") {
					dir := strings.TrimSpace(strings.TrimPrefix(line, "MODULESDIR:"))
					if dir != "" {
						for _, so := range candidates {
							if _, err := os.Stat(filepath.Join(dir, "vfs", so)); err == nil {
								return true
							}
							if _, err := os.Stat(filepath.Join(dir, so)); err == nil {
								return true
							}
						}
					}
				}
			}
		}
	}

	paths := []string{
		"/usr/lib/samba",
		"/usr/lib64/samba",
	}
	if runtime.GOARCH == "amd64" {
		paths = append(paths, "/usr/lib/x86_64-linux-gnu/samba")
	}
	if runtime.GOARCH == "arm64" {
		paths = append(paths, "/usr/lib/aarch64-linux-gnu/samba")
	}
	if runtime.GOARCH == "arm" {
		paths = append(paths, "/usr/lib/arm-linux-gnueabihf/samba")
	}

	for _, dir := range paths {
		for _, so := range candidates {
			if _, err := os.Stat(filepath.Join(dir, "vfs", so)); err == nil {
				return true
			}
			if _, err := os.Stat(filepath.Join(dir, so)); err == nil {
				return true
			}
		}
	}
	return false
}
