package web

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/version"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
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

type summaryRow struct {
	Label string
	Value string
}

type summaryView struct {
	Visible       bool
	BudgetWarning bool
	Rows          []summaryRow
}

func buildSummaryView(loc i18n.Localizer, s ocr.Summary) summaryView {
	if !s.Present() {
		return summaryView{}
	}
	v := summaryView{Visible: true}
	if s.BudgetExceeded != nil && *s.BudgetExceeded {
		v.BudgetWarning = true
	}
	addInt := func(key string, n *int) {
		if n == nil {
			return
		}
		v.Rows = append(v.Rows, summaryRow{Label: loc.T(key), Value: humanize.Comma(int64(*n))})
	}
	addInt64 := func(key string, n *int64) {
		if n == nil {
			return
		}
		v.Rows = append(v.Rows, summaryRow{Label: loc.T(key), Value: humanize.Comma(*n)})
	}
	addInt("run_detail.summary.files_reviewed", s.FilesReviewed)
	addInt("run_detail.summary.comments", s.Comments)
	addInt64("run_detail.summary.total_tokens", s.TotalTokens)
	addInt64("run_detail.summary.input_tokens", s.InputTokens)
	addInt64("run_detail.summary.output_tokens", s.OutputTokens)
	addInt64("run_detail.summary.cache_read_tokens", s.CacheReadTokens)
	if elapsed := strings.TrimSpace(s.Elapsed); elapsed != "" {
		v.Rows = append(v.Rows, summaryRow{Label: loc.T("run_detail.summary.elapsed"), Value: elapsed})
	}
	return v
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
		"shortSHA": version.ShortCommit,
	}).ParseFS(pagesFS, "templates/pages.html"),
)

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
