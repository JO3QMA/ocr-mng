package web

import (
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/store"
	"github.com/jo3qma/ocr-mng/internal/version"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

func testPage() page {
	loc := i18n.New("ja")
	return page{Title: "Dashboard", Lang: "ja", L: loc}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
	if got := formatTime((*time.Time)(nil)); got != "" {
		t.Fatalf("nil ptr: %q", got)
	}
	if got := formatTime(time.Time{}); got != "" {
		t.Fatalf("zero: %q", got)
	}
	ts := time.Date(2026, 7, 17, 12, 34, 0, 0, time.UTC)
	if got := formatTime(ts); got != "2026-07-17 12:34" {
		t.Fatalf("time: %q", got)
	}
	if got := formatTime(&ts); got != "2026-07-17 12:34" {
		t.Fatalf("ptr: %q", got)
	}
}

func TestFormatCommaInt64(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"0", "0"},
		{"999", "999"},
		{"1000", "1,000"},
		{"1344922", "1,344,922"},
		{"-42", "-42"},
		{fmt.Sprint(math.MinInt64), "-9,223,372,036,854,775,808"},
	} {
		var n int64
		if _, err := fmt.Sscanf(tc.in, "%d", &n); err != nil {
			t.Fatal(err)
		}
		if got := humanize.Comma(n); got != tc.want {
			t.Fatalf("humanize.Comma(%d) = %q, want %q", n, got, tc.want)
		}
	}
}

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

func TestRenderSettingsLLMRotationWidget(t *testing.T) {
	l := i18n.New("ja")
	widget := buildLLMRotationWidget(l, "default_llm_pairs", true, []llmPairOption{
		{Value: "1:2", Label: "A / m1"},
	}, []store.LLMPair{{ProviderID: 1, ModelID: 2}})
	rec := httptest.NewRecorder()
	render(rec, "settings", struct {
		page
		Settings    store.GlobalSettings
		LLMRotation llmRotationWidget
	}{
		page:        testPage(),
		Settings:    store.GlobalSettings{PollIntervalSeconds: 60},
		LLMRotation: widget,
	})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	for _, want := range []string{
		`class="llm-rotation"`,
		`name="default_llm_pairs"`,
		`data-llm-gated-save`,
		`llm-rotation-add-btn`,
		`<dialog class="llm-rotation-dialog"`,
		`llm-rotation-select-all`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body", want)
		}
	}
}

func TestRenderRunDetailRetryButton(t *testing.T) {
	run := store.ReviewRun{
		ID: 7, RepoID: 2, PRNumber: 11, Status: "failed",
		ErrorMessage: "ocr exited 1", RepoOwner: "acme", RepoName: "app",
	}
	rec := httptest.NewRecorder()
	render(rec, "run_detail", struct {
		page
		Run         store.ReviewRun
		SummaryView summaryView
		OCRJSON     string
	}{page: testPage(), Run: run})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, body)
	}
	for _, want := range []string{
		`action="/repos/2/review"`,
		`name="pr_number" value="11"`,
		`class="btn accent"`,
		"再実行",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %q", want, body)
		}
	}

	run.Status = "success"
	rec = httptest.NewRecorder()
	render(rec, "run_detail", struct {
		page
		Run         store.ReviewRun
		SummaryView summaryView
		OCRJSON     string
	}{page: testPage(), Run: run})
	if strings.Contains(rec.Body.String(), "再実行") {
		t.Fatalf("retry button on success: %q", rec.Body.String())
	}
}

func TestRenderRunDetailTimes(t *testing.T) {
	started := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 7, 17, 10, 5, 0, 0, time.UTC)
	run := store.ReviewRun{
		ID: 1, RepoID: 2, PRNumber: 3, Status: "success",
		CreatedAt: time.Date(2026, 7, 17, 9, 55, 0, 0, time.UTC),
		StartedAt: &started, FinishedAt: &finished,
		RepoOwner: "acme", RepoName: "app",
	}
	rec := httptest.NewRecorder()
	render(rec, "run_detail", struct {
		page
		Run         store.ReviewRun
		SummaryView summaryView
		OCRJSON     string
	}{page: testPage(), Run: run})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, body)
	}
	for _, want := range []string{"受付日時", "開始日時", "終了日時", "2026-07-17 09:55", "2026-07-17 10:00", "2026-07-17 10:05"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %q", want, body)
		}
	}
}

func TestRenderRunDetailSummary(t *testing.T) {
	files := 16
	tokens := int64(1344922)
	budget := true
	run := store.ReviewRun{ID: 1, PRNumber: 2, Status: "success", RepoOwner: "acme", RepoName: "app"}
	summary := ocr.Summary{
		FilesReviewed:  &files,
		TotalTokens:    &tokens,
		Elapsed:        "3m55s",
		BudgetExceeded: &budget,
	}
	rec := httptest.NewRecorder()
	render(rec, "run_detail", struct {
		page
		Run         store.ReviewRun
		SummaryView summaryView
		OCRJSON     string
	}{page: testPage(), Run: run, SummaryView: buildSummaryView(testPage().L, summary), OCRJSON: "{}"})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, body)
	}
	for _, want := range []string{
		"OCR レビュー統計",
		"レビューしたファイル数",
		"16",
		"1,344,922",
		"3m55s",
		"OCR のトークン予算を超過しました。",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %q", want, body)
		}
	}
}

func TestRenderRunDetailSummaryBudgetOnly(t *testing.T) {
	budget := true
	run := store.ReviewRun{ID: 1, PRNumber: 2, Status: "success", RepoOwner: "acme", RepoName: "app"}
	summary := ocr.Summary{BudgetExceeded: &budget}
	rec := httptest.NewRecorder()
	render(rec, "run_detail", struct {
		page
		Run         store.ReviewRun
		SummaryView summaryView
		OCRJSON     string
	}{page: testPage(), Run: run, SummaryView: buildSummaryView(testPage().L, summary), OCRJSON: "{}"})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, body)
	}
	if !strings.Contains(body, "OCR のトークン予算を超過しました。") {
		t.Fatalf("missing budget warning in %q", body)
	}
	if strings.Contains(body, "<table><tbody>\n</tbody></table>") {
		t.Fatalf("empty summary table rendered: %q", body)
	}
}

func TestRenderDashboard(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "dashboard", struct {
		page
		RepoCount int
		HostCount int
		Runs      any
	}{page: testPage(), RepoCount: 0, HostCount: 0, Runs: nil})
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestRenderHosts(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "hosts", struct {
		page
		Hosts any
	}{page: testPage(), Hosts: nil})
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRenderAbout(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "about", struct {
		page
		Info version.AboutInfo
	}{
		page: testPage(),
		Info: version.AboutInfo{
			ReviewManager:  "dev (abc1234)",
			DockerImageTag: "local",
			BaseImageFrom:  "debian:bookworm-slim",
			BaseImageOS:    "Debian GNU/Linux 12 (bookworm)",
			GitCLI:         "git version 2.43.0",
			OCRCLI:         "1.0.9 (abc1234)",
		},
	})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, body)
	}
	for _, want := range []string{"バージョン情報", "Review Manager", "dev (abc1234)", "debian:bookworm-slim", "git version 2.43.0"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body", want)
		}
	}
}
