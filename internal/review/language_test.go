package review_test

import (
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/review"
)

func TestForInlineJapanese(t *testing.T) {
	result := ocr.Result{
		Comments: []ocr.Comment{{FilePath: "main.go", StartLine: 1, Content: "fix"}},
	}
	_, summary := review.ForInline(result, review.CommentFormat{Lang: "Japanese", HostKind: "github"})
	if !strings.Contains(summary, "件のコメント") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestMergeOCRRequirement(t *testing.T) {
	got := review.MergeOCRRequirement("English", "focus on tests")
	if !strings.Contains(got, "path") || !strings.HasSuffix(got, "focus on tests") {
		t.Fatalf("merge: %q", got)
	}
	if !strings.HasPrefix(got, "Every comment must set path") {
		t.Fatalf("default should be first: %q", got)
	}
}

func TestMergeOCRRequirementRepoEmpty(t *testing.T) {
	got := review.MergeOCRRequirement("Japanese", "")
	if !strings.Contains(got, "path") {
		t.Fatalf("default only: %q", got)
	}
}
