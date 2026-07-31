package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizeReviewBackgroundFilePath trims and cleans a repo-root-relative path.
// Empty input clears the setting (returns "", nil). Invalid values return an error.
func NormalizeReviewBackgroundFilePath(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if strings.ContainsAny(s, "\n\r") {
		return "", fmt.Errorf("review background file path must not contain newlines")
	}
	if filepath.IsAbs(s) {
		return "", fmt.Errorf("review background file path must be relative to the repository root")
	}
	clean := filepath.Clean(s)
	if !filepath.IsLocal(clean) {
		return "", fmt.Errorf("review background file path must stay within the repository root")
	}
	return clean, nil
}

// ResolveReviewBackgroundFile returns the relative path to pass to OCR --background-file
// when worktreeDir contains a regular (non-symlink) file at relPath. Otherwise ok is false.
func ResolveReviewBackgroundFile(worktreeDir, relPath string) (pathForOCR string, ok bool) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", false
	}
	// Defense in depth if a legacy/invalid DB value slipped through.
	clean, err := NormalizeReviewBackgroundFilePath(relPath)
	if err != nil || clean == "" {
		return "", false
	}
	full := filepath.Join(worktreeDir, clean)
	absWork, err := filepath.Abs(worktreeDir)
	if err != nil {
		return "", false
	}
	absFile, err := filepath.Abs(full)
	if err != nil {
		return "", false
	}
	sep := string(os.PathSeparator)
	if absFile != absWork && !strings.HasPrefix(absFile, absWork+sep) {
		return "", false
	}
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	return clean, true
}
