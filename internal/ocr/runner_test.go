package ocr_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestReviewWithFakeBinary(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho '{\"comments\":[],\"message\":\"ok\"}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	result, raw, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "ok" || len(raw) == 0 {
		t.Fatalf("unexpected result: %+v raw=%q", result, raw)
	}
}
