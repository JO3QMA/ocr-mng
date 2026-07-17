package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jo3qma/ocr-mng/internal/store"
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
		Run     store.ReviewRun
		OCRJSON string
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
