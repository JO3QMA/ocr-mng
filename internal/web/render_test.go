package web

import (
	"net/http/httptest"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

func testPage() page {
	loc := i18n.New("ja")
	return page{Title: "Dashboard", Lang: "ja", L: loc}
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
