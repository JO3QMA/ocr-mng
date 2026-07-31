package ocr_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestTestLLMSuccess(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho '✓ Connection test successful'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := ocr.BuildProviderConfig("builtin", "anthropic", "sk-secret", "", "", "claude-x", "")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	runner := ocr.Runner{Binary: binary, HomeDir: home, ConfigJSON: cfg}
	if err := runner.TestLLM(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".opencodereview", "config.json")); err != nil {
		t.Fatal(err)
	}
}

func TestTestLLMFailureMasksNothingHere(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho 'auth failed sk-secret' >&2; exit 1\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := ocr.Runner{
		Binary: binary, HomeDir: t.TempDir(),
		ConfigJSON: `{"provider":"anthropic","model":"m","providers":{"anthropic":{"api_key":"sk-secret","model":"m","models":["m"]}}}`,
	}
	err := runner.TestLLM(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	masked := ocr.MaskSecret(err.Error(), "sk-secret")
	if strings.Contains(masked, "sk-secret") {
		t.Fatalf("secret leaked: %q", masked)
	}
	if !strings.Contains(masked, "***") {
		t.Fatalf("expected mask: %q", masked)
	}
}

func TestTestLLMTimeout(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\nexec sleep 30\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	runner := ocr.Runner{
		Binary: binary, HomeDir: t.TempDir(),
		ConfigJSON: `{"provider":"anthropic","model":"m","providers":{"anthropic":{"api_key":"k","model":"m","models":["m"]}}}`,
	}
	err := runner.TestLLM(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestMaskSecret(t *testing.T) {
	if got := ocr.MaskSecret("x", ""); got != "x" {
		t.Fatalf("empty secret: %q", got)
	}
	if got := ocr.MaskSecret("use sk-abc now sk-abc", "sk-abc"); got != "use *** now ***" {
		t.Fatalf("got %q", got)
	}
}
