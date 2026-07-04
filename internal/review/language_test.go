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
	_, summary := review.ForInline(result, "Japanese")
	if !strings.Contains(summary, "件のコメント") {
		t.Fatalf("summary: %q", summary)
	}
}
