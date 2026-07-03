package githost_test

import (
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
