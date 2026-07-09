package review

import (
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

const apiGoDiff = "diff --git a/internal/githost/api.go b/internal/githost/api.go\n" +
	"--- a/internal/githost/api.go\n" +
	"+++ b/internal/githost/api.go\n" +
	"@@ -104,6 +104,9 @@\n" +
	" \t}\n" +
	" \tcommentsPayload := make([]payloadComment, 0, len(comments))\n" +
	" \tfor _, cm := range comments {\n" +
	"+\t\tif cm.Path == \"\" {\n" +
	"+\t\t\tcontinue\n" +
	"+\t\t}\n" +
	" \t\tline := cm.Line\n"

func TestParseDiffLines_apiGoHunk(t *testing.T) {
	lines := ParseDiffLines(apiGoDiff)
	path := "internal/githost/api.go"
	if lines[path] == nil {
		t.Fatalf("missing path %q", path)
	}
	if _, ok := lines[path][107]; !ok {
		t.Fatalf("expected line 107 in diff, got %v", lines[path])
	}
	if _, ok := lines[path][114]; ok {
		t.Fatal("line 114 should not be in short hunk")
	}
}

func TestClampToDiff(t *testing.T) {
	diff := ParseDiffLines("diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n+++ b/a.go\n" +
		"@@ -10,3 +10,5 @@\n" +
		" context\n+added\n more\n")
	_, _, ok := clampToDiff("a.go", 1, 9, diff)
	if ok {
		t.Fatal("expected no intersection")
	}
	line, start, ok := clampToDiff("a.go", 10, 12, diff)
	if !ok || line != 12 || start != 10 {
		t.Fatalf("clamp: line=%d start=%d ok=%v", line, start, ok)
	}
	line, start, ok = clampToDiff("a.go", 11, 11, diff)
	if !ok || line != 11 || start != 0 {
		t.Fatalf("single line: line=%d start=%d ok=%v", line, start, ok)
	}
}

func TestForInlineDiffClamp(t *testing.T) {
	inline, summary, demoted := ForInline(ocr.Result{
		Comments: []ocr.Comment{{
			FilePath: "internal/githost/api.go", StartLine: 105, EndLine: 114,
			Content: "validate path",
		}},
	}, CommentFormat{Lang: "English", HostKind: "github"}, ParseDiffLines(apiGoDiff))
	if len(inline) != 1 {
		t.Fatalf("inline: %+v", inline)
	}
	if inline[0].Line != 110 || inline[0].StartLine != 105 {
		t.Fatalf("clamped anchor: %+v", inline[0])
	}
	if len(demoted) != 0 {
		t.Fatalf("demoted: %v", demoted)
	}
	if !strings.Contains(summary, "Found **1** comment") {
		t.Fatalf("summary: %q", summary)
	}
}
