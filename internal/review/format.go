package review

import (
	"fmt"
	"path/filepath"
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

func commentBody(c ocr.Comment, cf CommentFormat, w wrapperMsgs, asInlineComment bool) string {
	return c.Content + formatSuggestion(c, cf, w, asInlineComment)
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
	fmt.Fprintf(&b, w.foundComments, len(result.Comments))
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
