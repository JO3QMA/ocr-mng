package ocr

import (
	"fmt"
	"sort"
	"strings"
)

// ParseExtraHeadersLines parses extra HTTP headers from textarea lines "key=value".
// Blank lines are ignored. Duplicate keys are an error.
func ParseExtraHeadersLines(raw string) (map[string]string, error) {
	out := make(map[string]string)
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.ContainsAny(line, "\r") {
			return nil, fmt.Errorf("header line %d: invalid characters", i+1)
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("header line %d: expected key=value", i+1)
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf("header line %d: empty key", i+1)
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("header line %d: invalid characters", i+1)
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate header %q", key)
		}
		out[key] = value
	}
	return out, nil
}

// FormatExtraHeadersLines renders headers as sorted key=value lines for forms.
func FormatExtraHeadersLines(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(h[k])
	}
	return b.String()
}
