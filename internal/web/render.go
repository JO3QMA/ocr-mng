package web

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/pages.html
var pagesFS embed.FS

// formatTime formats a time.Time or *time.Time for UI. Nil / zero → empty string.
func formatTime(v any) string {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02 15:04")
	case *time.Time:
		if t == nil || t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02 15:04")
	default:
		return ""
	}
}

func formatCommaInt64(n int64) string {
	s := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign = "-"
		s = s[1:]
	}
	if len(s) <= 3 {
		return sign + s
	}
	var b strings.Builder
	b.Grow(len(s) + (len(s)-1)/3 + len(sign))
	b.WriteString(sign)
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

var pageTemplates = template.Must(
	template.New("").Funcs(template.FuncMap{
		"formatTime": formatTime,
		"selected": func(got, want string) template.HTMLAttr {
			if got == want || (got == "" && want == "github") || (got == "" && want == "inline") {
				return `selected`
			}
			return ""
		},
		"selectedInt": func(got, want int64) template.HTMLAttr {
			if got == want {
				return `selected`
			}
			return ""
		},
		"checked": func(v bool) template.HTMLAttr {
			if v {
				return `checked`
			}
			return ""
		},
		"defaultStr": func(v, fallback string) string {
			if v == "" {
				return fallback
			}
			return v
		},
		"shortSHA": func(s string) string {
			if len(s) > 8 {
				return s[:8]
			}
			return s
		},
	}).ParseFS(pagesFS, "templates/pages.html"),
)

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
