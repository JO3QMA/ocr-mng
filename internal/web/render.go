package web

import (
	"embed"
	"html/template"
	"net/http"
	"time"
)

//go:embed templates/pages.html
var pagesFS embed.FS

var pageTemplates = template.Must(
	template.New("").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02 15:04")
		},
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
		"yesNo": func(v bool) string {
			if v {
				return "yes"
			}
			return "no"
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
