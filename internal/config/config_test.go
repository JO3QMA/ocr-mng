package config_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/config"
)

func TestLoad(t *testing.T) {
	t.Setenv("RM_ENCRYPTION_KEY", "01234567890123456789012345678901")
	t.Setenv("RM_ADMIN_USER", "admin")
	t.Setenv("RM_ADMIN_PASSWORD", "secret")
	t.Setenv("RM_LISTEN_ADDR", ":9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":9090" || cfg.AdminUser != "admin" || cfg.OCRBinary != "ocr" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRequiresEnv(t *testing.T) {
	t.Setenv("RM_ENCRYPTION_KEY", "")
	t.Setenv("RM_ADMIN_USER", "")
	t.Setenv("RM_ADMIN_PASSWORD", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error")
	}
}
