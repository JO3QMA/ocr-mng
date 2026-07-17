package ocr

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildProviderConfig builds a minimal OCR config.json for one provider + one model.
// kind is "builtin" (providers.*) or "custom" (custom_providers.*).
// custom requires apiBaseURL; protocol may be omitted and is then inferred from the URL.
func BuildProviderConfig(kind, providerKey, apiKey, apiBaseURL, protocol, model, language string) (string, error) {
	providerKey = strings.TrimSpace(providerKey)
	model = strings.TrimSpace(model)
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		protocol = InferProtocol(apiBaseURL)
	}
	if providerKey == "" || model == "" {
		return "", fmt.Errorf("provider key and model are required")
	}
	if protocol != "" && !ValidProtocol(protocol) {
		return "", fmt.Errorf("protocol must be anthropic, openai, or openai-responses")
	}
	entry := map[string]any{
		"api_key": apiKey,
		"model":   model,
		"models":  []string{model},
	}
	if apiBaseURL != "" {
		entry["url"] = apiBaseURL
	}
	if protocol != "" {
		entry["protocol"] = protocol
	}
	m := map[string]any{
		"provider": providerKey,
		"model":    model,
	}
	switch kind {
	case "builtin":
		m["providers"] = map[string]any{providerKey: entry}
	case "custom":
		if apiBaseURL == "" {
			return "", fmt.Errorf("api base url is required for custom providers")
		}
		if protocol == "" {
			return "", fmt.Errorf("protocol is required for custom providers")
		}
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
