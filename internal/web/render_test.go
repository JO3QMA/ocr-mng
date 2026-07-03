package web

import (
	"net/http/httptest"
	"testing"
)

func TestRenderDashboard(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "dashboard", map[string]any{
		"Title": "Dashboard",
		"Repos": nil,
		"Runs":  nil,
	})
	if rec.Code != 200 {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestRenderHosts(t *testing.T) {
	rec := httptest.NewRecorder()
	render(rec, "hosts", map[string]any{
		"Title": "Git Hosts",
		"Hosts": nil,
	})
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
}
