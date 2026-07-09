package review_test

import (
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestForInlineJapanese(t *testing.T) {
	result := ocr.Result{
		Comments: []ocr.Comment{{FilePath: "main.go", StartLine: 1, Content: "fix"}},
	}
	_, summary := review.ForInline(result, review.CommentFormat{Lang: "Japanese", HostKind: "github"})
	if !strings.Contains(summary, "件のコメント") {
		t.Fatalf("summary: %q", summary)
	}
}

func TestMergeOCRRequirement(t *testing.T) {
	got := review.MergeOCRRequirement("English", "focus on tests")
	if !strings.Contains(got, "path") || !strings.HasSuffix(got, "focus on tests") {
		t.Fatalf("merge: %q", got)
	}
	if !strings.HasPrefix(got, "Every comment must set path") {
		t.Fatalf("default should be first: %q", got)
	}
}

func TestZeroFindingApprovalEnabled(t *testing.T) {
	repo := store.RepoView{
		Repo: store.Repo{ApproveOnZeroFindings: true},
		HostKind: "github",
	}
	if !review.ZeroFindingApprovalEnabled(repo, 0) {
		t.Fatal("expected enabled for zero findings on github")
	}
	if review.ZeroFindingApprovalEnabled(repo, 1) {
		t.Fatal("expected disabled when comments exist")
	}
	repo.HostKind = "gitea"
	if review.ZeroFindingApprovalEnabled(repo, 0) {
		t.Fatal("expected disabled on gitea")
	}
}

func TestApprovalBody(t *testing.T) {
	if review.ApprovalBody("Japanese") == "" || review.ApprovalBody("English") == "" {
		t.Fatal("expected non-empty approval bodies")
	}
}

func TestMergeOCRRequirementRepoEmpty(t *testing.T) {
	got := review.MergeOCRRequirement("Japanese", "")
	if !strings.Contains(got, "path") {
		t.Fatalf("default only: %q", got)
	}
}
