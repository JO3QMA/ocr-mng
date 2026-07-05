package review_test

import (
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/review"
)

func englishFmt() review.CommentFormat {
	return review.CommentFormat{Lang: "English", HostKind: "github"}
}

func TestForInline(t *testing.T) {
	result := ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "main.go", StartLine: 1, EndLine: 2, Content: "fix me",
		}},
	}
	inline, summary := review.ForInline(result, englishFmt())
	if len(inline) != 1 || inline[0].Line != 2 {
		t.Fatalf("inline: %+v", inline)
	}
	if !strings.Contains(summary, "Found **1** comment") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestForInlineNoComments(t *testing.T) {
	_, summary := review.ForInline(ocr.Result{Message: "clean"}, englishFmt())
	if !strings.Contains(summary, "clean") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestAsSingleCommentWithLine(t *testing.T) {
	body := review.AsSingleComment(ocr.Result{
		Comments: []ocr.Comment{{FilePath: "a.go", StartLine: 3, Content: "note"}},
	}, englishFmt())
	if !strings.Contains(body, "a.go:3") || !strings.Contains(body, "note") {
		t.Fatalf("body: %q", body)
	}
}

func TestAsSingleCommentUnresolvedFilePath(t *testing.T) {
	body := review.AsSingleComment(ocr.Result{
		Comments: []ocr.Comment{{StartLine: 630, Content: "fix path"}},
	}, englishFmt())
	if strings.Contains(body, "(general)") || !strings.Contains(body, "(file unknown):630") {
		t.Fatalf("body: %q", body)
	}
}

func TestAsSingleCommentGeneralWithoutLine(t *testing.T) {
	body := review.AsSingleComment(ocr.Result{
		Comments: []ocr.Comment{{Content: "overall"}},
	}, englishFmt())
	if !strings.Contains(body, "### (general)\n") {
		t.Fatalf("body: %q", body)
	}
}

func TestForInlineSkipsEmptyPathWithLine(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{StartLine: 10, Content: "orphan line"}},
	}, englishFmt())
	if len(inline) != 0 {
		t.Fatalf("inline: %+v", inline)
	}
	if !strings.Contains(summary, "(file unknown):10") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestCommentBodyGitHubSuggestion(t *testing.T) {
	inline, _ := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "frontend/src/VrcUserCacheDetail.vue", StartLine: 153, EndLine: 158,
			Content: "check attrs",
			Suggestion: " <VrcUserTagChip\n class=\"tag-chip\"\n />\n",
		}},
	}, review.CommentFormat{Lang: "English", HostKind: "github"})
	if len(inline) != 1 {
		t.Fatalf("inline: %+v", inline)
	}
	body := inline[0].Body
	if strings.Contains(body, "**Suggestion:**") {
		t.Fatalf("github inline should omit label: %q", body)
	}
	if !strings.Contains(body, "```suggestion\n <VrcUserTagChip\n class=\"tag-chip\"\n />\n```") {
		t.Fatalf("body: %q", body)
	}
}

func TestCommentBodyFallbackFence(t *testing.T) {
	body := review.AsSingleComment(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "main.go", StartLine: 1, Content: "use fmt",
			Suggestion: "fmt.Println(\"hi\")\n",
		}},
	}, review.CommentFormat{Lang: "English", HostKind: "gitea"})
	if !strings.Contains(body, "**Suggestion:**") {
		t.Fatalf("fallback should keep label: %q", body)
	}
	if !strings.Contains(body, "```go\nfmt.Println(\"hi\")\n```") {
		t.Fatalf("body: %q", body)
	}
}

func TestCommentBodyTrimsTrailingNewline(t *testing.T) {
	inline, _ := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Suggestion: "x = 1\n\n",
		}},
	}, englishFmt())
	if strings.Contains(inline[0].Body, "x = 1\n\n```") {
		t.Fatalf("trailing newlines should be trimmed: %q", inline[0].Body)
	}
}
