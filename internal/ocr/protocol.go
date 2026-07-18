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

// AbsoluteAPIBaseURL adds https:// when the value looks like a host/path without a scheme.
// Unparseable or empty input is returned trimmed as-is (or empty).
func AbsoluteAPIBaseURL(apiBaseURL string) string {
	raw := strings.TrimSpace(apiBaseURL)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Host != "" {
		return raw
	}
	// url.Parse treats "api.openai.com/v1" as a path with empty Host.
	u, err = url.Parse("https://" + raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.String()
}

// InferProtocol guesses an OCR protocol from an API base URL.
// Empty or unparseable URL → empty. Path with /v1/responses → openai-responses.
// Host mentioning anthropic → anthropic. Otherwise openai (Chat Completions).
// Explicit Protocol on the provider always wins over this guess at save time.
func InferProtocol(apiBaseURL string) string {
	raw := AbsoluteAPIBaseURL(apiBaseURL)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	path := strings.ToLower(u.EscapedPath())
	if looksLikeResponsesPath(path) {
		return ProtocolOpenAIResponses
	}
	host := strings.ToLower(u.Hostname())
	// ponytail: substring match covers custom proxies (my-anthropic-proxy); wrong guess → set Protocol explicitly.
	if strings.Contains(host, "anthropic") {
		return ProtocolAnthropic
	}
	return ProtocolOpenAI
}

func looksLikeResponsesPath(path string) bool {
	path = strings.TrimSuffix(path, "/")
	return path == "/responses" || strings.HasSuffix(path, "/v1/responses") || strings.Contains(path, "/v1/responses/")
}
