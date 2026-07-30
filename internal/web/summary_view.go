package web

import (
	"strings"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

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
		v.Rows = append(v.Rows, summaryRow{Label: loc.T(key), Value: formatCommaInt64(int64(*n))})
	}
	addInt64 := func(key string, n *int64) {
		if n == nil {
			return
		}
		v.Rows = append(v.Rows, summaryRow{Label: loc.T(key), Value: formatCommaInt64(*n)})
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
