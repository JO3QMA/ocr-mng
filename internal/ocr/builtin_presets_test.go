package ocr_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestBuiltinPresetLabel(t *testing.T) {
	label, ok := ocr.BuiltinPresetLabel("anthropic")
	if !ok || label != "Anthropic" {
		t.Fatalf("got %q %v", label, ok)
	}
	if _, ok := ocr.BuiltinPresetLabel("not-a-provider"); ok {
		t.Fatal("expected miss")
	}
}

func TestSelectedBuiltinPreset(t *testing.T) {
	if got := ocr.SelectedBuiltinPreset("deepseek"); got != "deepseek" {
		t.Fatalf("got %q", got)
	}
	if got := ocr.SelectedBuiltinPreset("my-gateway"); got != ocr.BuiltinPresetOther {
		t.Fatalf("got %q", got)
	}
}

func TestBuiltinProviderDocsURL(t *testing.T) {
	if got := ocr.BuiltinProviderDocsURL("ja"); got != "https://open-codereview.ai/docs/ja/configuration" {
		t.Fatalf("ja: %q", got)
	}
	if got := ocr.BuiltinProviderDocsURL("JA"); got != "https://open-codereview.ai/docs/ja/configuration" {
		t.Fatalf("JA: %q", got)
	}
	if got := ocr.BuiltinProviderDocsURL("ja-JP"); got != "https://open-codereview.ai/docs/ja/configuration" {
		t.Fatalf("ja-JP: %q", got)
	}
	if got := ocr.BuiltinProviderDocsURL("en"); got != "https://open-codereview.ai/docs/configuration" {
		t.Fatalf("en: %q", got)
	}
}
