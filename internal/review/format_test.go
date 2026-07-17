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

func TestCommentBodyEscapesTripleBackticks(t *testing.T) {
	inline, _ := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Suggestion: "x := ````",
		}},
	}, englishFmt())
	if strings.Contains(inline[0].Body, "```suggestion\nx := ```") {
		t.Fatalf("triple backticks should be escaped: %q", inline[0].Body)
	}
	if !strings.Contains(inline[0].Body, "x := \\`\\`\\`") {
		t.Fatalf("body: %q", inline[0].Body)
	}
}

func TestCommentBodyPreservesLeadingTab(t *testing.T) {
	inline, _ := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "internal/application/auction/preview_usecase.go", StartLine: 25, EndLine: 25,
			Content: "remove json tag",
			Suggestion: "\tMarketEstimate *domainmarket.MarketEstimate",
		}},
	}, englishFmt())
	if len(inline) != 1 {
		t.Fatalf("inline: %+v", inline)
	}
	want := "```suggestion\n\tMarketEstimate *domainmarket.MarketEstimate\n```"
	if !strings.Contains(inline[0].Body, want) {
		t.Fatalf("leading tab should be preserved: %q", inline[0].Body)
	}
}

func TestCommentBodyPreservesTrailingTab(t *testing.T) {
	inline, _ := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Suggestion: "if x {\n\treturn 1\n\t",
		}},
	}, englishFmt())
	if len(inline) != 1 {
		t.Fatalf("inline: %+v", inline)
	}
	want := "```suggestion\nif x {\n\treturn 1\n\t\n```"
	if !strings.Contains(inline[0].Body, want) {
		t.Fatalf("trailing tab should be preserved: %q", inline[0].Body)
	}
}

func TestCommentBodyTrimsLeadingNewline(t *testing.T) {
	inline, _ := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Suggestion: "\nx = 1",
		}},
	}, englishFmt())
	if strings.Contains(inline[0].Body, "```suggestion\n\n") {
		t.Fatalf("leading newline should be trimmed: %q", inline[0].Body)
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

func TestCommentMetaBoth(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Severity: "medium", Category: "maintainability",
		}},
	}, englishFmt())
	wantMeta := "**Severity:** medium · **Category:** maintainability\n\nfix"
	if !strings.HasPrefix(inline[0].Body, wantMeta) {
		t.Fatalf("body: %q", inline[0].Body)
	}
	if !strings.Contains(summary, "**Severity:** medium 1\n") {
		t.Fatalf("summary severity: %q", summary)
	}
	if !strings.Contains(summary, "**Category:** maintainability 1\n") {
		t.Fatalf("summary category: %q", summary)
	}
}

func TestCommentMetaPartial(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{
			{FilePath: "a.go", StartLine: 1, Content: "sev only", Severity: "low"},
			{FilePath: "b.go", StartLine: 2, Content: "cat only", Category: "style"},
		},
	}, englishFmt())
	if !strings.HasPrefix(inline[0].Body, "**Severity:** low\n\nsev only") {
		t.Fatalf("sev body: %q", inline[0].Body)
	}
	if !strings.HasPrefix(inline[1].Body, "**Category:** style\n\ncat only") {
		t.Fatalf("cat body: %q", inline[1].Body)
	}
	if !strings.Contains(summary, "**Severity:** low 1\n") || !strings.Contains(summary, "**Category:** style 1\n") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestCommentMetaAbsent(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{FilePath: "a.go", StartLine: 1, Content: "legacy"}},
	}, englishFmt())
	if strings.Contains(inline[0].Body, "**Severity:**") || strings.Contains(inline[0].Body, "**Category:**") {
		t.Fatalf("body should omit meta: %q", inline[0].Body)
	}
	if strings.Contains(summary, "**Severity:**") || strings.Contains(summary, "**Category:**") {
		t.Fatalf("summary should omit breakdown: %q", summary)
	}
	if !strings.HasPrefix(inline[0].Body, "legacy") {
		t.Fatalf("body: %q", inline[0].Body)
	}
}

func TestCommentMetaTrim(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Severity: " medium ", Category: "  ",
		}},
	}, englishFmt())
	if !strings.HasPrefix(inline[0].Body, "**Severity:** medium\n\nfix") {
		t.Fatalf("body: %q", inline[0].Body)
	}
	if strings.Contains(inline[0].Body, "**Category:**") {
		t.Fatalf("blank category omitted: %q", inline[0].Body)
	}
	if !strings.Contains(summary, "**Severity:** medium 1\n") || strings.Contains(summary, "**Category:**") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestCommentBreakdownSort(t *testing.T) {
	_, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{
			{FilePath: "a.go", StartLine: 1, Content: "1", Severity: "low", Category: "style"},
			{FilePath: "b.go", StartLine: 1, Content: "2", Severity: "medium", Category: "maintainability"},
			{FilePath: "c.go", StartLine: 1, Content: "3", Severity: "medium", Category: "maintainability"},
		},
	}, englishFmt())
	if !strings.Contains(summary, "**Severity:** medium 2, low 1\n") {
		t.Fatalf("severity order: %q", summary)
	}
	if !strings.Contains(summary, "**Category:** maintainability 2, style 1\n") {
		t.Fatalf("category order: %q", summary)
	}
}

func TestAsSingleCommentSummary(t *testing.T) {
	body := review.AsSingleComment(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 3, Content: "note",
			Severity: "high", Category: "security",
		}},
	}, englishFmt())
	if !strings.Contains(body, "Found **1** comment(s).\n") {
		t.Fatalf("count line: %q", body)
	}
	if !strings.Contains(body, "**Severity:** high 1\n") || !strings.Contains(body, "**Category:** security 1\n") {
		t.Fatalf("breakdown: %q", body)
	}
	if !strings.Contains(body, "**Severity:** high · **Category:** security\n\nnote") {
		t.Fatalf("meta: %q", body)
	}
}

func TestCommentMetaJapaneseLabels(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "指摘",
			Severity: "low", Category: "style",
		}},
	}, review.CommentFormat{Lang: "Japanese", HostKind: "github"})
	if !strings.HasPrefix(inline[0].Body, "**深刻度:** low · **分類:** style\n\n指摘") {
		t.Fatalf("body: %q", inline[0].Body)
	}
	if !strings.Contains(summary, "**深刻度:** low 1\n") || !strings.Contains(summary, "**分類:** style 1\n") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestCommentMetaEscapesMarkdown(t *testing.T) {
	inline, summary := review.ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "a.go", StartLine: 1, Content: "fix",
			Severity: "*high*", Category: "style_x",
		}},
	}, englishFmt())
	if !strings.HasPrefix(inline[0].Body, "**Severity:** \\*high\\* · **Category:** style\\_x\n\nfix") {
		t.Fatalf("body: %q", inline[0].Body)
	}
	if !strings.Contains(summary, "**Severity:** \\*high\\* 1\n") {
		t.Fatalf("summary: %q", summary)
	}
	if !strings.Contains(summary, "**Category:** style\\_x 1\n") {
		t.Fatalf("summary category: %q", summary)
	}
}
