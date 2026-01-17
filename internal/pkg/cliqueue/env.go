package cliqueue

import (
	"os"
	"strconv"
	"strings"
)

func ParseEnvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func ClampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
