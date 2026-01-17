package file

import (
	"os"
	"path/filepath"
	"strings"
)

func ReadString(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(value)
}

func ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				p = home
			} else if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	p = os.ExpandEnv(p)
	return filepath.Clean(p)
}
