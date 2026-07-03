package config

import (
	"fmt"
	"os"
)

type Config struct {
	ListenAddr    string
	DataDir       string
	EncryptionKey []byte
	AdminUser     string
	AdminPassword string
	OCRBinary     string
}

func Load() (Config, error) {
	key := os.Getenv("RM_ENCRYPTION_KEY")
	if len(key) < 32 {
		return Config{}, fmt.Errorf("RM_ENCRYPTION_KEY must be at least 32 bytes")
	}
	user := os.Getenv("RM_ADMIN_USER")
	pass := os.Getenv("RM_ADMIN_PASSWORD")
	if user == "" || pass == "" {
		return Config{}, fmt.Errorf("RM_ADMIN_USER and RM_ADMIN_PASSWORD are required")
	}
	return Config{
		ListenAddr:    envOr("RM_LISTEN_ADDR", ":8080"),
		DataDir:       envOr("RM_DATA_DIR", "/data"),
		EncryptionKey: []byte(key),
		AdminUser:     user,
		AdminPassword: pass,
		OCRBinary:     envOr("RM_OCR_BINARY", "ocr"),
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
