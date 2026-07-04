package i18n_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

func TestLocalizerJapanese(t *testing.T) {
	loc := i18n.New("ja")
	if got := loc.T("nav.dashboard"); got != "ダッシュボード" {
		t.Fatalf("got %q", got)
	}
}

func TestLocalizerFallbackEnglish(t *testing.T) {
	loc := i18n.New("en")
	if got := loc.T("nav.dashboard"); got != "Dashboard" {
		t.Fatalf("got %q", got)
	}
}
