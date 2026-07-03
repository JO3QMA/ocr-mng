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

func commentBody(c ocr.Comment) string {
	body := c.Content
	if c.Suggestion != "" {
		body += "\n\n**Suggestion:** " + c.Suggestion
	}
	if c.Priority != "" {
		body = fmt.Sprintf("[%s] %s", c.Priority, body)
	}
	return body
}

// ForInline splits OCR output into inline review comments and a summary markdown body.
func ForInline(result ocr.Result) ([]githost.ReviewComment, string) {
	var inline []githost.ReviewComment
	var b strings.Builder
	b.WriteString("## Open Code Review\n\n")
	if len(result.Comments) == 0 {
		msg := result.Message
		if msg == "" {
			msg = "No comments generated."
		}
		fmt.Fprintf(&b, "✅ %s\n", msg)
		return inline, b.String()
	}
	fmt.Fprintf(&b, "Found **%d** comment(s).\n\n", len(result.Comments))
	for _, c := range result.Comments {
		line := commentLine(c)
		if line >= 1 {
			inline = append(inline, githost.ReviewComment{
				Path: c.FilePath, Line: line, StartLine: c.StartLine, Body: commentBody(c),
			})
			continue
		}
		title := c.FilePath
		if title == "" {
			title = "(general)"
		}
		fmt.Fprintf(&b, "#### %s\n%s\n\n", title, commentBody(c))
	}
	if len(result.Warnings) > 0 {
		b.WriteString("### Warnings\n")
		for _, w := range result.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	return inline, b.String()
}

// AsSingleComment renders all OCR comments as one issue comment body.
func AsSingleComment(result ocr.Result) string {
	var b strings.Builder
	b.WriteString("## Open Code Review\n\n")
	if len(result.Comments) == 0 {
		msg := result.Message
		if msg == "" {
			msg = "No comments generated."
		}
		fmt.Fprintf(&b, "✅ %s\n", msg)
		return b.String()
	}
	for _, c := range result.Comments {
		title := c.FilePath
		if title == "" {
			title = "(general)"
		}
		if line := commentLine(c); line > 0 {
			fmt.Fprintf(&b, "### %s:%d\n", title, line)
		} else {
			fmt.Fprintf(&b, "### %s\n", title)
		}
		fmt.Fprintf(&b, "%s\n\n", commentBody(c))
	}
	return b.String()
}
