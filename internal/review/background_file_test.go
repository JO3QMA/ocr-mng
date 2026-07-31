package review_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/review"
)

func TestNormalizeReviewBackgroundFilePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"./docs/REVIEW_CONTEXT.md", "docs/REVIEW_CONTEXT.md", false},
		{"docs/../README.md", "README.md", false},
		{"docs/foo.md\n", "docs/foo.md", false}, // trailing newline trimmed
		{"/etc/passwd", "", true},
		{"../../outside.md", "", true},
		{"docs/foo.md\nbad", "", true},
		{"docs/foo.md\rbar", "", true},
		{"docs/foo\x00.md", "", true},
	}
	for _, tc := range cases {
		got, err := review.NormalizeReviewBackgroundFilePath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveReviewBackgroundFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := "docs/ctx.md"
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, rel)
	if err := os.WriteFile(full, []byte("# ctx"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := review.ResolveReviewBackgroundFile(root, rel)
	if !ok || got != rel {
		t.Fatalf("regular file: got %q ok=%v", got, ok)
	}
	if _, ok := review.ResolveReviewBackgroundFile(root, "missing.md"); ok {
		t.Fatal("missing should skip")
	}
	if _, ok := review.ResolveReviewBackgroundFile(root, "docs"); ok {
		t.Fatal("directory should skip")
	}

	link := filepath.Join(root, "link.md")
	if err := os.Symlink(full, link); err != nil {
		t.Fatal(err)
	}
	if _, ok := review.ResolveReviewBackgroundFile(root, "link.md"); ok {
		t.Fatal("symlink should skip")
	}

	// Intermediate directory symlink escaping the worktree must be rejected.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	escapeDir := filepath.Join(root, "escape")
	if err := os.Symlink(outside, escapeDir); err != nil {
		t.Fatal(err)
	}
	if _, ok := review.ResolveReviewBackgroundFile(root, "escape/secret.md"); ok {
		t.Fatal("path via intermediate symlink outside worktree should skip")
	}
}
