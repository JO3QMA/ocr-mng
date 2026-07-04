package ocr

import (
	"encoding/json"
	"strings"
)

// ConfigWithLanguage merges language into OCR global config JSON.
// The language argument always wins over any existing language key.
func ConfigWithLanguage(configJSON, language string) (string, error) {
	m := map[string]any{}
	raw := strings.TrimSpace(configJSON)
	if raw != "" && raw != "{}" {
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return "", err
		}
	}
	if language != "" {
		m["language"] = language
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
