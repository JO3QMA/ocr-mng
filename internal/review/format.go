package review

import (
	"fmt"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/githost"
	"github.com/jo3qma/ocr-mng/internal/ocr"
)

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

func writeSummaryHeading(b *strings.Builder, c ocr.Comment, w wrapperMsgs) {
	title := commentTitle(c, w)
	if line := commentLine(c); line >= 1 {
		fmt.Fprintf(b, "#### %s:%d\n%s\n\n", title, line, commentBody(c, w))
		return
	}
	fmt.Fprintf(b, "#### %s\n%s\n\n", title, commentBody(c, w))
}

func commentBody(c ocr.Comment, w wrapperMsgs) string {
	body := c.Content
	if c.Suggestion != "" {
		body += "\n\n" + w.suggestion + c.Suggestion
	}
	if c.Priority != "" {
		body = fmt.Sprintf("[%s] %s", c.Priority, body)
	}
	return body
}

// ForInline splits OCR output into inline review comments and a summary markdown body.
func ForInline(result ocr.Result, lang string) ([]githost.ReviewComment, string) {
	w := wrapperFor(lang)
	var inline []githost.ReviewComment
	var b strings.Builder
	b.WriteString(w.title)
	if len(result.Comments) == 0 {
		msg := result.Message
		if msg == "" {
			msg = w.noComments
		}
		fmt.Fprintf(&b, "✅ %s\n", msg)
		return inline, b.String()
	}
	fmt.Fprintf(&b, w.foundComments, len(result.Comments))
	for _, c := range result.Comments {
		line := commentLine(c)
		if line >= 1 && c.FilePath != "" {
			inline = append(inline, githost.ReviewComment{
				Path: c.FilePath, Line: line, StartLine: c.StartLine, Body: commentBody(c, w),
			})
			continue
		}
		writeSummaryHeading(&b, c, w)
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
func AsSingleComment(result ocr.Result, lang string) string {
	w := wrapperFor(lang)
	var b strings.Builder
	b.WriteString(w.title)
	if len(result.Comments) == 0 {
		msg := result.Message
		if msg == "" {
			msg = w.noComments
		}
		fmt.Fprintf(&b, "✅ %s\n", msg)
		return b.String()
	}
	for _, c := range result.Comments {
		title := commentTitle(c, w)
		if line := commentLine(c); line > 0 {
			fmt.Fprintf(&b, "### %s:%d\n", title, line)
		} else {
			fmt.Fprintf(&b, "### %s\n", title)
		}
		fmt.Fprintf(&b, "%s\n\n", commentBody(c, w))
	}
	return b.String()
}
