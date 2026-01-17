package graph

import (
	"os"
	"strconv"
	"strings"
	"unicode"
)

func isDeviceRemovable(src string) bool {
	if !strings.HasPrefix(src, "/dev/") {
		return false
	}
	base := strings.TrimPrefix(src, "/dev/")
	dev := base
	if strings.HasPrefix(base, "nvme") {
		i := strings.LastIndexFunc(base, func(r rune) bool { return !unicode.IsDigit(r) })
		if i >= 0 && i+1 < len(base) && base[i] == 'p' {
			j := i
			for j >= 0 && base[j] != 'n' {
				j--
			}
			if j >= 0 {
				dev = base[:i]
			}
		}
	} else {
		i := len(base) - 1
		for i >= 0 && unicode.IsDigit(rune(base[i])) {
			i--
		}
		dev = base[:i+1]
	}

	paths := []string{
		"/sys/block/" + dev + "/removable",
		"/sys/block/" + dev + "/device/removable",
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			s := strings.TrimSpace(string(data))
			if s == "1" || strings.EqualFold(s, "true") {
				return true
			}
		}
	}
	return false
}

func parseMeminfoKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func charsToString(ca [65]int8) string {
	n := make([]byte, 0, len(ca))
	for _, v := range ca {
		if v == 0 {
			break
		}
		n = append(n, byte(v))
	}
	return string(n)
}
