package ocr_test

import (
	"encoding/json"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestBuildProviderConfig_builtin(t *testing.T) {
	out, err := ocr.BuildProviderConfig("builtin", "anthropic", "sk-secret", "", "", "claude-sonnet-4-6", "Japanese")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["provider"] != "anthropic" || m["model"] != "claude-sonnet-4-6" || m["language"] != "Japanese" {
		t.Fatalf("top: %+v", m)
	}
	providers, ok := m["providers"].(map[string]any)
	if !ok {
		t.Fatalf("providers: %T", m["providers"])
	}
	entry, ok := providers["anthropic"].(map[string]any)
	if !ok {
		t.Fatalf("entry: %+v", providers)
	}
	if entry["api_key"] != "sk-secret" || entry["model"] != "claude-sonnet-4-6" {
		t.Fatalf("entry: %+v", entry)
	}
	if _, has := m["custom_providers"]; has {
		t.Fatal("unexpected custom_providers")
	}
}

func TestBuildProviderConfig_custom(t *testing.T) {
	out, err := ocr.BuildProviderConfig("custom", "my-gw", "tok", "https://example/v1", "openai", "gpt-x", "English")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	customs := m["custom_providers"].(map[string]any)
	entry := customs["my-gw"].(map[string]any)
	if entry["url"] != "https://example/v1" || entry["protocol"] != "openai" {
		t.Fatalf("entry: %+v", entry)
	}
	if _, has := m["providers"]; has {
		t.Fatal("unexpected providers")
	}
}

func TestNeedsOpenCodeGoSessionHeader(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://opencode.ai/zen/go/v1/responses", true},
		{"https://opencode.ai/zen/go/v1/chat/completions", true},
		{"opencode.ai/zen/go/v1/responses", true},
		{"https://opencode.ai/zen/v1/chat/completions", false},
		{"https://api.openai.com/v1", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ocr.NeedsOpenCodeGoSessionHeader(tc.url); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.url, got, tc.want)
		}
	}
}

func TestBuildProviderConfig_openCodeGoAddsSessionHeader(t *testing.T) {
	out, err := ocr.BuildProviderConfig(
		"custom", "opencode-go", "sk-go", "https://opencode.ai/zen/go/v1/responses",
		"openai-responses", "muse-spark-1.3-contributor", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	entry := m["custom_providers"].(map[string]any)["opencode-go"].(map[string]any)
	headers, ok := entry["extra_headers"].(map[string]any)
	if !ok {
		t.Fatalf("extra_headers: %T %+v", entry["extra_headers"], entry)
	}
	if headers["x-opencode-session"] != "{ocr_session_key}" {
		t.Fatalf("headers: %+v", headers)
	}
}

func TestBuildProviderConfig_customRequiresURLAndProtocol(t *testing.T) {
	if _, err := ocr.BuildProviderConfig("custom", "my-gw", "tok", "", "openai", "gpt-x", ""); err == nil {
		t.Fatal("expected url required")
	}
	out, err := ocr.BuildProviderConfig("custom", "my-gw", "tok", "https://example/v1", "", "gpt-x", "")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	entry := m["custom_providers"].(map[string]any)["my-gw"].(map[string]any)
	if entry["protocol"] != "openai" {
		t.Fatalf("expected inferred openai: %+v", entry)
	}
	if _, err := ocr.BuildProviderConfig("custom", "my-gw", "tok", "https://example/v1", "bogus", "gpt-x", ""); err == nil {
		t.Fatal("expected invalid protocol")
	}
}
