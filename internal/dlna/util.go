package dlna

import (
	"encoding/xml"
	"strings"
)

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func dlnaDebugEnabled() bool {
	return true
}

func dlnaDebugPayloadEnabled() bool {
	return true
}

func truncateForLog(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
