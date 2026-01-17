package config

import (
	"bufio"
	"ismartcoding/plainnas/internal/consts"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	values map[string]any
}

var configMain = &Config{values: map[string]any{}}

func GetDefault() *Config {
	return configMain
}

func Load() {
	f, err := os.Open(consts.ETC_MAIN_CONFIG)
	if err != nil {
		configMain = &Config{values: map[string]any{}}
		return
	}
	defer f.Close()

	configMain = &Config{values: parseSimpleTOML(f)}
}

func (c *Config) GetString(key string) string {
	if c == nil {
		return ""
	}
	v, ok := c.values[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func (c *Config) GetInt(key string) int {
	if c == nil {
		return 0
	}
	v, ok := c.values[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(x))
		return i
	default:
		return 0
	}
}

func parseSimpleTOML(r *os.File) map[string]any {
	values := map[string]any{}
	section := ""

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = stripTOMLComment(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		raw := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		fullKey := key
		if section != "" {
			fullKey = section + "." + key
		}

		values[fullKey] = parseTOMLScalar(raw)
	}

	return values
}

func stripTOMLComment(line string) string {
	inQuotes := false
	escaped := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inQuotes {
			escaped = true
			continue
		}
		if ch == '"' {
			inQuotes = !inQuotes
			continue
		}
		if ch == '#' && !inQuotes {
			return line[:i]
		}
	}
	return line
}

func parseTOMLScalar(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "\"") {
		if s, err := strconv.Unquote(raw); err == nil {
			return s
		}
		return strings.Trim(raw, "\"")
	}
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2 {
		return raw[1 : len(raw)-1]
	}
	if raw == "true" {
		return true
	}
	if raw == "false" {
		return false
	}
	if i, err := strconv.Atoi(raw); err == nil {
		return i
	}
	return raw
}

func GetNasName() string {
	return GetDefault().GetString("nas.name")
}
