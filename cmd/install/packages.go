package install

import (
	"log"
	"os/exec"

	pkgcmd "ismartcoding/plainnas/internal/pkg/cmd"
)

type installPlan struct {
	name           string
	presentAny     []string
	presentAll     []string
	aptPkg         string
	dnfPkg         string
	yumPkg         string
	pacmanPkg      string
	apkPkg         string
	noSupportedMsg string
}

func ensureInstalled(plan installPlan) {
	if hasAnyInPath(plan.presentAny) || hasAllInPath(plan.presentAll) {
		if plan.name != "" {
			printOK(plan.name, "Already installed")
		}
		return
	}

	label := plan.name
	if label == "" {
		label = "Install packages"
	}

	// Prefer apt-get over apt if both exist.
	switch detectPkgManager() {
	case "apt-get":
		if plan.aptPkg == "" {
			printNote(label, plan.noSupportedMsg)
			return
		}
		printNote(label, "Installing…")
		_ = pkgcmd.RunProgress("Update package index", "DEBIAN_FRONTEND=noninteractive apt-get update")
		if err := pkgcmd.RunProgress(label, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+plan.aptPkg); err != nil {
			printFail(label, "Install failed. You can try installing it manually.")
			log.Println(err)
			return
		}
		printOK(label, "Installed")
	case "apt":
		if plan.aptPkg == "" {
			printNote(label, plan.noSupportedMsg)
			return
		}
		printNote(label, "Installing…")
		_ = pkgcmd.RunProgress("Update package index", "DEBIAN_FRONTEND=noninteractive apt update")
		if err := pkgcmd.RunProgress(label, "DEBIAN_FRONTEND=noninteractive apt install -y "+plan.aptPkg); err != nil {
			printFail(label, "Install failed. You can try installing it manually.")
			log.Println(err)
			return
		}
		printOK(label, "Installed")
	case "dnf":
		if plan.dnfPkg == "" {
			printNote(label, plan.noSupportedMsg)
			return
		}
		printNote(label, "Installing…")
		if err := pkgcmd.RunProgress(label, "dnf install -y "+plan.dnfPkg); err != nil {
			printFail(label, "Install failed. You can try installing it manually.")
			log.Println(err)
			return
		}
		printOK(label, "Installed")
	case "yum":
		if plan.yumPkg == "" {
			printNote(label, plan.noSupportedMsg)
			return
		}
		printNote(label, "Installing…")
		if err := pkgcmd.RunProgress(label, "yum install -y "+plan.yumPkg); err != nil {
			printFail(label, "Install failed. You can try installing it manually.")
			log.Println(err)
			return
		}
		printOK(label, "Installed")
	case "pacman":
		if plan.pacmanPkg == "" {
			printNote(label, plan.noSupportedMsg)
			return
		}
		printNote(label, "Installing…")
		if err := pkgcmd.RunProgress(label, "pacman -Sy --noconfirm "+plan.pacmanPkg); err != nil {
			printFail(label, "Install failed. You can try installing it manually.")
			log.Println(err)
			return
		}
		printOK(label, "Installed")
	case "apk":
		if plan.apkPkg == "" {
			printNote(label, plan.noSupportedMsg)
			return
		}
		printNote(label, "Installing…")
		if err := pkgcmd.RunProgress(label, "apk add --no-cache "+plan.apkPkg); err != nil {
			printFail(label, "Install failed. You can try installing it manually.")
			log.Println(err)
			return
		}
		printOK(label, "Installed")
	default:
		printNote(label, plan.noSupportedMsg)
	}
}

func hasAnyInPath(names []string) bool {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func hasAllInPath(names []string) bool {
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return false
		}
	}
	return true
}

func detectPkgManager() string {
	switch {
	case hasCmd("apt-get"):
		return "apt-get"
	case hasCmd("apt"):
		return "apt"
	case hasCmd("dnf"):
		return "dnf"
	case hasCmd("yum"):
		return "yum"
	case hasCmd("pacman"):
		return "pacman"
	case hasCmd("apk"):
		return "apk"
	default:
		return ""
	}
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
