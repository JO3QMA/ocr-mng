package review_test

import (
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/review"
)

func TestForInline(t *testing.T) {
	result := ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "main.go", StartLine: 1, EndLine: 2, Content: "fix me", Priority: "P1",
		}},
	}
	inline, summary := review.ForInline(result, "English")
	if len(inline) != 1 || inline[0].Line != 2 {
		t.Fatalf("inline: %+v", inline)
	}
	if !strings.Contains(summary, "Found **1** comment") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestForInlineNoComments(t *testing.T) {
	_, summary := review.ForInline(ocr.Result{Message: "clean"}, "English")
	if !strings.Contains(summary, "clean") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestAsSingleCommentWithLine(t *testing.T) {
	body := review.AsSingleComment(ocr.Result{
		Comments: []ocr.Comment{{FilePath: "a.go", StartLine: 3, Content: "note"}},
	}, "English")
	if !strings.Contains(body, "a.go:3") || !strings.Contains(body, "note") {
		t.Fatalf("body: %q", body)
	}
}
