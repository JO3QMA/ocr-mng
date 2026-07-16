package ocr

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildProviderConfig builds a minimal OCR config.json for one provider + one model.
// kind is "builtin" (providers.*) or "custom" (custom_providers.*).
func BuildProviderConfig(kind, providerKey, apiKey, apiBaseURL, protocol, model, language string) (string, error) {
	providerKey = strings.TrimSpace(providerKey)
	model = strings.TrimSpace(model)
	if providerKey == "" || model == "" {
		return "", fmt.Errorf("provider key and model are required")
	}
	entry := map[string]any{
		"api_key": apiKey,
		"model":   model,
		"models":  []string{model},
	}
	if u := strings.TrimSpace(apiBaseURL); u != "" {
		entry["url"] = u
	}
	if p := strings.TrimSpace(protocol); p != "" {
		entry["protocol"] = p
	}
	m := map[string]any{
		"provider": providerKey,
		"model":    model,
	}
	switch kind {
	case "builtin":
		m["providers"] = map[string]any{providerKey: entry}
	case "custom":
		m["custom_providers"] = map[string]any{providerKey: entry}
	default:
		return "", fmt.Errorf("kind must be builtin or custom")
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
