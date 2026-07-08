package githost_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/githost"
)

func TestHasLabel(t *testing.T) {
	labels := []string{"review", "bug"}
	if !githost.HasLabel(labels, "review") {
		t.Fatal("expected review label")
	}
	if githost.HasLabel(labels, "feature") {
		t.Fatal("unexpected label")
	}
}

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
	prs, err := c.ListOpenPullRequests(context.Background(), "o", "r")
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
	if !githost.HasLabel(pr.Labels, "review") {
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
	prs, err := c.ListOpenPullRequests(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if prs[0].Title != "T" || prs[0].Body != "" {
		t.Fatalf("pr: %+v", prs[0])
	}
}
