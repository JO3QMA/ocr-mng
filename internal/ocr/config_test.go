package ocr_test

import (
	"encoding/json"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestConfigWithLanguage(t *testing.T) {
	out, err := ocr.ConfigWithLanguage(`{"llm":{"model":"x"},"language":"English"}`, "Japanese")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["language"] != "Japanese" {
		t.Fatalf("language: %v", m["language"])
	}
}
