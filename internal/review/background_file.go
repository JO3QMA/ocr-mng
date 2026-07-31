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
// when worktreeDir contains a regular (non-symlink) file at relPath whose fully resolved
// path stays under the worktree. Otherwise ok is false.
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
	// Reject final-component symlinks (and non-files) before resolving parents.
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", false
	}
	absWork, err := filepath.EvalSymlinks(worktreeDir)
	if err != nil {
		return "", false
	}
	absWork, err = filepath.Abs(absWork)
	if err != nil {
		return "", false
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absWork, absResolved)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return "", false
	}
	return clean, true
}
