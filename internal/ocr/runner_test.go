package ocr_test

import (
	"context"
	"encoding/json"
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

func TestReviewPassesBackground(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\ncase \"$*\" in *--background*) echo '{\"comments\":[],\"message\":\"ok\"}' ;; *) echo 'missing background' >&2; exit 1 ;; esac\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	if _, _, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "need path"); err != nil {
		t.Fatal(err)
	}
}

func TestCommentJSONUsesOCRPathKey(t *testing.T) {
	var result ocr.Result
	raw := `{"comments":[{"path":"packages/backend/src/foo.ts","content":"fix","suggestion_code":"bar","start_line":156,"end_line":159}]}`
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	c := result.Comments[0]
	if c.FilePath != "packages/backend/src/foo.ts" || c.Suggestion != "bar" || c.StartLine != 156 {
		t.Fatalf("comment: %+v", c)
	}
}
