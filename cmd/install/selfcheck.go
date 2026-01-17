package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pkgcmd "ismartcoding/plainnas/internal/pkg/cmd"
)

func runSelfChecks() {
	printSection("Quick checks")
	selfCheckLibvips()
	selfCheckFFmpeg()
	selfCheckSamba()
	selfCheckAvahi()
}

func selfCheckFFmpeg() {
	okFFmpeg, verFFmpeg := hasBinaryVersion("ffmpeg", "-version")
	okFFprobe, _ := hasBinaryVersion("ffprobe", "-version")
	if okFFmpeg && okFFprobe {
		printOK("Video thumbnails", verFFmpeg)
		return
	}
	printNote("Video thumbnails", "Not fully installed. Videos may not get thumbnails.")
}

func selfCheckLibvips() {
	okVips, verVips := hasBinaryVersion("vips", "--version")
	okThumb, _ := hasBinaryVersion("vipsthumbnail", "--version")
	if okVips && okThumb {
		printOK("Photo thumbnails", verVips)
		return
	}
	printNote("Photo thumbnails", "Not fully installed. Photos may be slower to preview.")
}

func selfCheckSamba() {
	okSmbd, _ := hasBinaryVersion("smbd", "-V")
	okSmbpasswd, _ := hasBinaryVersion("smbpasswd", "-V")
	if !okSmbd || !okSmbpasswd {
		printNote("LAN sharing (Windows/macOS)", "Not installed. You can still use PlainNAS locally.")
		return
	}

	if hasSambaVfsModule("fruit") {
		printOK("macOS Finder support", "Enabled")
	} else {
		printNote("macOS Finder support", "Limited. If macOS shows errors, install 'samba-vfs-modules'.")
	}

	if _, err := exec.LookPath("systemctl"); err == nil {
		active := strings.TrimSpace(string(execQuiet("systemctl", "is-active", "smbd")))
		if active == "active" {
			printOK("Samba service", "Running")
		} else if active != "" && active != "unknown" {
			printNote("Samba service", "Not running. Enable it in Settings → LAN Share.")
		}
	}
}

func selfCheckAvahi() {
	ok, _ := hasBinaryVersion("avahi-daemon", "-V")
	if ok {
		printOK("Local name (.local)", "Enabled")
		return
	}
	printNote("Local name (.local)", "Not installed. You can always use the NAS IP address.")
}

func hasBinaryVersion(bin string, versionArg string) (bool, string) {
	p, err := exec.LookPath(bin)
	if err != nil {
		return false, ""
	}
	out, err := pkgcmd.Exec(bin, versionArg)
	if err != nil {
		return true, filepath.Base(p)
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	if first == "" {
		return true, filepath.Base(p)
	}
	return true, first
}

func hasSambaVfsModule(name string) bool {
	out := execQuiet("smbd", "-b")
	modulesDir := ""
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MODULESDIR:") {
			modulesDir = strings.TrimSpace(strings.TrimPrefix(line, "MODULESDIR:"))
			break
		}
	}
	if modulesDir == "" {
		return false
	}
	for _, so := range []string{name + ".so", "vfs_" + name + ".so"} {
		if _, err := os.Stat(filepath.Join(modulesDir, "vfs", so)); err == nil {
			return true
		}
	}
	return false
}

func execQuiet(name string, arg ...string) []byte {
	out, _ := pkgcmd.Exec(name, arg...)
	return out
}
