package install

import (
	"os/exec"
	"strings"

	pkgcmd "ismartcoding/plainnas/internal/pkg/cmd"
)

func installAllDeps() {
	printSection("Installing dependencies")
	installLibvips()
	installFFmpeg()
	installSamba()
	installAvahi()
}

func installLibreOffice() {
	ensureInstalled(installPlan{
		name:           "DOC/DOCX preview (LibreOffice)",
		presentAny:     []string{"soffice", "libreoffice"},
		aptPkg:         "libreoffice",
		dnfPkg:         "libreoffice",
		yumPkg:         "libreoffice",
		pacmanPkg:      "libreoffice-fresh",
		apkPkg:         "libreoffice",
		noSupportedMsg: "Couldn't detect a supported package manager to install LibreOffice.",
	})
}

func installAvahi() {
	ensureInstalled(installPlan{
		name:           "Local name discovery (.local)",
		presentAny:     []string{"avahi-daemon", "avahi-browse"},
		aptPkg:         "avahi-daemon avahi-utils libnss-mdns",
		dnfPkg:         "avahi avahi-tools nss-mdns",
		yumPkg:         "avahi avahi-tools nss-mdns",
		pacmanPkg:      "avahi nss-mdns",
		apkPkg:         "avahi avahi-tools",
		noSupportedMsg: "Couldn't detect a supported package manager to install Avahi.",
	})

	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = pkgcmd.RunProgress("Enable & start avahi-daemon", "systemctl enable --now avahi-daemon")
	}
}

func installSamba() {
	ensureInstalled(installPlan{
		name:           "LAN sharing (Samba)",
		presentAll:     []string{"smbd", "smbpasswd"},
		aptPkg:         "samba",
		dnfPkg:         "samba",
		yumPkg:         "samba",
		pacmanPkg:      "samba",
		apkPkg:         "samba",
		noSupportedMsg: "Couldn't detect a supported package manager to install Samba.",
	})
	ensureSambaVfsModulesDebianUbuntu()
}

func ensureSambaVfsModulesDebianUbuntu() {
	switch detectPkgManager() {
	case "apt-get", "apt":
		if !hasCmd("dpkg-query") {
			return
		}
		out, err := pkgcmd.Exec("dpkg-query", "-W", "-f=${Status}", "samba-vfs-modules")
		if err == nil && strings.Contains(string(out), "install ok installed") {
			return
		}
		_ = pkgcmd.RunProgress(
			"Install macOS Samba compatibility",
			"DEBIAN_FRONTEND=noninteractive apt-get install -y samba-vfs-modules",
		)
	default:
		return
	}
}

func installFFmpeg() {
	ensureInstalled(installPlan{
		name:           "Video thumbnails (ffmpeg)",
		presentAll:     []string{"ffmpeg", "ffprobe"},
		aptPkg:         "ffmpeg",
		dnfPkg:         "ffmpeg",
		yumPkg:         "ffmpeg",
		pacmanPkg:      "ffmpeg",
		apkPkg:         "ffmpeg",
		noSupportedMsg: "Couldn't detect a supported package manager to install ffmpeg.",
	})
}

func installLibvips() {
	ensureInstalled(installPlan{
		name:           "Photo thumbnails (libvips)",
		presentAny:     []string{"vipsthumbnail", "vips"},
		aptPkg:         "libvips-tools",
		dnfPkg:         "vips",
		yumPkg:         "vips",
		pacmanPkg:      "vips",
		apkPkg:         "vips",
		noSupportedMsg: "Couldn't detect a supported package manager to install libvips.",
	})
}
