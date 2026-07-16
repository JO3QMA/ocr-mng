package githost_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/githost"
)

func TestNewClientAuth(t *testing.T) {
	gh := githost.New("github", "https://api.github.com/", "https://github.com/")
	gt := githost.New("gitea", "https://git.example/api/v1/", "https://git.example/")
	if gh.CloneURL("o", "r", "pat") == gt.CloneURL("o", "r", "pat") {
		t.Fatal("expected different clone URLs per host kind")
	}
}

func TestCloneURLWithoutPAT(t *testing.T) {
	c := githost.New("github", "https://api.github.com", "https://github.com")
	got := c.CloneURL("owner", "repo", "")
	want := "https://github.com/owner/repo.git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestListOpenPullRequestsTitleBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/pulls" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"number": 42,
			"title": "Fix bug",
			"body": "Details here",
			"base": {"ref": "main"},
			"head": {"sha": "abc123"},
			"labels": [{"name": "review"}]
		}]`))
	}))
	defer srv.Close()

	c := githost.New("github", srv.URL, "https://github.com")
	prs, err := c.ListOpenPullRequests(context.Background(), "", "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 {
		t.Fatalf("len=%d", len(prs))
	}
	pr := prs[0]
	if pr.Number != 42 || pr.Title != "Fix bug" || pr.Body != "Details here" {
		t.Fatalf("pr: %+v", pr)
	}
	if pr.BaseRef != "main" || pr.HeadSHA != "abc123" {
		t.Fatalf("refs: %+v", pr)
	}
	if !slices.Contains(pr.Labels, "review") {
		t.Fatal("expected review label")
	}
}

func TestListOpenPullRequestsGiteaTitleBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("limit param: %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"number": 1, "title": "T", "body": "", "base": {"ref": "dev"}, "head": {"sha": "deadbeef"}, "labels": []}]`))
	}))
	defer srv.Close()

	c := githost.New("gitea", srv.URL, "https://git.example")
	prs, err := c.ListOpenPullRequests(context.Background(), "", "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if prs[0].Title != "T" || prs[0].Body != "" {
		t.Fatalf("pr: %+v", prs[0])
	}
}

func TestGetPullRequestOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/pulls/42" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"state": "open",
			"number": 42,
			"title": "Fix bug",
			"body": "Details",
			"base": {"ref": "main"},
			"head": {"sha": "abc123"},
			"labels": [{"name": "review"}]
		}`))
	}))
	defer srv.Close()

	c := githost.New("github", srv.URL, "https://github.com")
	pr, err := c.GetPullRequest(context.Background(), "", "o", "r", 42)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 42 || pr.HeadSHA != "abc123" {
		t.Fatalf("pr: %+v", pr)
	}
}

func TestGetPullRequestClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state": "closed", "number": 7, "title": "", "body": "", "base": {"ref": "main"}, "head": {"sha": "x"}, "labels": []}`))
	}))
	defer srv.Close()

	c := githost.New("github", srv.URL, "https://github.com")
	_, err := c.GetPullRequest(context.Background(), "", "o", "r", 7)
	if err == nil {
		t.Fatal("expected error for closed PR")
	}
}
