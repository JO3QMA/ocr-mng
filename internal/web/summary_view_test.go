package web

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }
func boolPtr(v bool) *bool    { return &v }

func TestBuildSummaryView(t *testing.T) {
	loc := i18n.New("ja")
	tests := []struct {
		name        string
		summary     ocr.Summary
		wantVisible bool
		wantBudget  bool
		wantRows    []summaryRow
	}{
		{
			name:        "empty",
			summary:     ocr.Summary{},
			wantVisible: false,
		},
		{
			name: "all fields",
			summary: ocr.Summary{
				FilesReviewed:   intPtr(16),
				Comments:        intPtr(6),
				TotalTokens:     int64Ptr(1344922),
				InputTokens:     int64Ptr(1269674),
				OutputTokens:    int64Ptr(75248),
				CacheReadTokens: int64Ptr(1077120),
				Elapsed:         "3m55s",
			},
			wantVisible: true,
			wantRows: []summaryRow{
				{loc.T("run_detail.summary.files_reviewed"), "16"},
				{loc.T("run_detail.summary.comments"), "6"},
				{loc.T("run_detail.summary.total_tokens"), "1,344,922"},
				{loc.T("run_detail.summary.input_tokens"), "1,269,674"},
				{loc.T("run_detail.summary.output_tokens"), "75,248"},
				{loc.T("run_detail.summary.cache_read_tokens"), "1,077,120"},
				{loc.T("run_detail.summary.elapsed"), "3m55s"},
			},
		},
		{
			name:        "budget only",
			summary:     ocr.Summary{BudgetExceeded: boolPtr(true)},
			wantVisible: true,
			wantBudget:  true,
		},
		{
			name: "elapsed whitespace only",
			summary: ocr.Summary{
				FilesReviewed: intPtr(1),
				Elapsed:       "   ",
			},
			wantVisible: true,
			wantRows: []summaryRow{
				{loc.T("run_detail.summary.files_reviewed"), "1"},
			},
		},
		{
			name: "partial fields",
			summary: ocr.Summary{
				TotalTokens: int64Ptr(100),
			},
			wantVisible: true,
			wantRows: []summaryRow{
				{loc.T("run_detail.summary.total_tokens"), "100"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSummaryView(loc, tc.summary)
			if got.Visible != tc.wantVisible {
				t.Fatalf("Visible = %v, want %v", got.Visible, tc.wantVisible)
			}
			if got.BudgetWarning != tc.wantBudget {
				t.Fatalf("BudgetWarning = %v, want %v", got.BudgetWarning, tc.wantBudget)
			}
			if len(got.Rows) != len(tc.wantRows) {
				t.Fatalf("Rows = %+v, want %+v", got.Rows, tc.wantRows)
			}
			for i := range tc.wantRows {
				if got.Rows[i] != tc.wantRows[i] {
					t.Fatalf("Rows[%d] = %+v, want %+v", i, got.Rows[i], tc.wantRows[i])
				}
			}
		})
	}
}
