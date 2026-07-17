package review

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/githost"
	"github.com/jo3qma/ocr-mng/internal/ocr"
)

// CommentFormat controls how OCR comments are rendered for a Git Host.
type CommentFormat struct {
	Lang     string
	HostKind string // "github" or "gitea"
}

var fenceLangByExt = map[string]string{
	".go":   "go",
	".ts":   "typescript",
	".tsx":  "tsx",
	".js":   "javascript",
	".jsx":  "jsx",
	".py":   "python",
	".rb":   "ruby",
	".java": "java",
	".kt":   "kotlin",
	".swift": "swift",
	".cs":   "csharp",
	".php":  "php",
	".rs":   "rust",
	".vue":  "vue",
	".html": "html",
	".css":  "css",
	".scss": "scss",
	".sql":  "sql",
	".sh":   "bash",
	".yaml": "yaml",
	".yml":  "yaml",
	".json": "json",
	".md":   "markdown",
	".c":    "c",
	".cpp":  "cpp",
	".h":    "c",
	".hpp":  "cpp",
	".xml":  "xml",
	".toml": "toml",
}

func commentLine(c ocr.Comment) int {
	if c.EndLine >= 1 {
		return c.EndLine
	}
	return c.StartLine
}

func commentTitle(c ocr.Comment, w wrapperMsgs) string {
	if c.FilePath != "" {
		return c.FilePath
	}
	if commentLine(c) >= 1 {
		return w.unknownFile
	}
	return w.general
}

func fenceLang(path string) string {
	return fenceLangByExt[strings.ToLower(filepath.Ext(path))]
}

// trimSuggestion strips only leading/trailing newlines so Markdown fences
// stay valid. Indentation (spaces, tabs) is preserved verbatim.
func trimSuggestion(code string) string {
	return strings.Trim(code, "\n\r")
}

func escapeFenceBreakers(code string) string {
	return strings.ReplaceAll(code, "```", "\\`\\`\\`")
}

func formatSuggestion(c ocr.Comment, cf CommentFormat, w wrapperMsgs, asInlineComment bool) string {
	code := escapeFenceBreakers(trimSuggestion(c.Suggestion))
	if code == "" {
		return ""
	}
	if asInlineComment && cf.HostKind == "github" {
		return "\n\n```suggestion\n" + code + "\n```"
	}
	lang := fenceLang(c.FilePath)
	if lang != "" {
		return "\n\n" + w.suggestion + "\n```" + lang + "\n" + code + "\n```"
	}
	return "\n\n" + w.suggestion + "\n```\n" + code + "\n```"
}

func commentMetaValue(s string) string {
	return strings.TrimSpace(s)
}

func formatMetaLine(c ocr.Comment, w wrapperMsgs) string {
	sev := commentMetaValue(c.Severity)
	cat := commentMetaValue(c.Category)
	var parts []string
	if sev != "" {
		parts = append(parts, "**"+w.severityLabel+":** "+sev)
	}
	if cat != "" {
		parts = append(parts, "**"+w.categoryLabel+":** "+cat)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ") + "\n\n"
}

func commentBody(c ocr.Comment, cf CommentFormat, w wrapperMsgs, asInlineComment bool) string {
	return formatMetaLine(c, w) + c.Content + formatSuggestion(c, cf, w, asInlineComment)
}

func tallyMeta(comments []ocr.Comment) (severities, categories map[string]int) {
	severities = make(map[string]int)
	categories = make(map[string]int)
	for _, c := range comments {
		if sev := commentMetaValue(c.Severity); sev != "" {
			severities[sev]++
		}
		if cat := commentMetaValue(c.Category); cat != "" {
			categories[cat]++
		}
	}
	return severities, categories
}

func formatCountPairs(counts map[string]int) string {
	type pair struct {
		key   string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for k, n := range counts {
		pairs = append(pairs, pair{k, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].key < pairs[j].key
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = fmt.Sprintf("%s %d", p.key, p.count)
	}
	return strings.Join(parts, ", ")
}

func writeCommentsHeader(b *strings.Builder, comments []ocr.Comment, w wrapperMsgs) {
	fmt.Fprintf(b, w.foundComments+"\n", len(comments))
	severities, categories := tallyMeta(comments)
	if len(severities) > 0 {
		fmt.Fprintf(b, "**%s:** %s\n", w.severityLabel, formatCountPairs(severities))
	}
	if len(categories) > 0 {
		fmt.Fprintf(b, "**%s:** %s\n", w.categoryLabel, formatCountPairs(categories))
	}
	b.WriteString("\n")
}

func writeSummaryHeading(b *strings.Builder, c ocr.Comment, cf CommentFormat, w wrapperMsgs) {
	title := commentTitle(c, w)
	if line := commentLine(c); line >= 1 {
		fmt.Fprintf(b, "#### %s:%d\n%s\n\n", title, line, commentBody(c, cf, w, false))
		return
	}
	fmt.Fprintf(b, "#### %s\n%s\n\n", title, commentBody(c, cf, w, false))
}

func writeZeroCommentBody(b *strings.Builder, result ocr.Result, w wrapperMsgs) {
	msg := result.Message
	if msg == "" {
		msg = w.noComments
	}
	fmt.Fprintf(b, "✅ %s\n", msg)
}

// ForInline splits OCR output into inline review comments and a summary markdown body.
func ForInline(result ocr.Result, cf CommentFormat) ([]githost.ReviewComment, string) {
	w := wrapperFor(cf.Lang)
	var inline []githost.ReviewComment
	var b strings.Builder
	b.WriteString(w.title)
	if len(result.Comments) == 0 {
		writeZeroCommentBody(&b, result, w)
		return inline, b.String()
	}
	writeCommentsHeader(&b, result.Comments, w)
	for _, c := range result.Comments {
		line := commentLine(c)
		if line >= 1 && c.FilePath != "" {
			inline = append(inline, githost.ReviewComment{
				Path: c.FilePath, Line: line, StartLine: c.StartLine,
				Body: commentBody(c, cf, w, true),
			})
			continue
		}
		writeSummaryHeading(&b, c, cf, w)
	}
	if len(result.Warnings) > 0 {
		b.WriteString(w.warnings)
		for _, warn := range result.Warnings {
			fmt.Fprintf(&b, "- %s\n", warn)
		}
	}
	return inline, b.String()
}

// AsSingleComment renders all OCR comments as one issue comment body.
func AsSingleComment(result ocr.Result, cf CommentFormat) string {
	w := wrapperFor(cf.Lang)
	var b strings.Builder
	b.WriteString(w.title)
	if len(result.Comments) == 0 {
		writeZeroCommentBody(&b, result, w)
		return b.String()
	}
	writeCommentsHeader(&b, result.Comments, w)
	for _, c := range result.Comments {
		title := commentTitle(c, w)
		if line := commentLine(c); line > 0 {
			fmt.Fprintf(&b, "### %s:%d\n", title, line)
		} else {
			fmt.Fprintf(&b, "### %s\n", title)
		}
		fmt.Fprintf(&b, "%s\n\n", commentBody(c, cf, w, false))
	}
	return b.String()
}
