package ocr

import (
	"net/url"
	"strings"
)

// OCR-supported LLM protocols (see alibaba/open-code-review llm.ValidateProtocol).
const (
	ProtocolAnthropic       = "anthropic"
	ProtocolOpenAI          = "openai"
	ProtocolOpenAIResponses = "openai-responses"
)

// ValidProtocol reports whether p is a canonical OCR protocol name.
func ValidProtocol(p string) bool {
	switch p {
	case ProtocolAnthropic, ProtocolOpenAI, ProtocolOpenAIResponses:
		return true
	default:
		return false
	}
}

// InferProtocol guesses an OCR protocol from an API base URL.
// Empty URL → empty. Path with /responses → openai-responses.
// Host mentioning anthropic → anthropic. Otherwise openai (Chat Completions).
func InferProtocol(apiBaseURL string) string {
	raw := strings.TrimSpace(apiBaseURL)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// bare path / typo: fall back to substring checks
		lower := strings.ToLower(raw)
		if strings.Contains(lower, "/responses") {
			return ProtocolOpenAIResponses
		}
		if strings.Contains(lower, "anthropic") {
			return ProtocolAnthropic
		}
		return ProtocolOpenAI
	}
	path := strings.ToLower(u.EscapedPath())
	if strings.Contains(path, "/responses") {
		return ProtocolOpenAIResponses
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, "anthropic") {
		return ProtocolAnthropic
	}
	return ProtocolOpenAI
}
