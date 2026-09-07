package ocr_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestParseExtraHeadersLines(t *testing.T) {
	got, err := ocr.ParseExtraHeadersLines("x-opencode-session={ocr_session_key}\n\nx-foo=bar")
	if err != nil {
		t.Fatal(err)
	}
	if got["x-opencode-session"] != "{ocr_session_key}" || got["x-foo"] != "bar" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseExtraHeadersLinesErrors(t *testing.T) {
	cases := []string{
		"noequals",
		"=value",
		"x-opencode-session=a\nx-opencode-session=b",
		"bad\rkey=value",
	}
	for _, raw := range cases {
		if _, err := ocr.ParseExtraHeadersLines(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestFormatExtraHeadersLines(t *testing.T) {
	got := ocr.FormatExtraHeadersLines(map[string]string{
		"z-last": "1",
		"a-first": "2",
	})
	want := "a-first=2\nz-last=1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
