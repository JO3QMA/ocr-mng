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

func TestBuildReviewBackgroundMergeOrder(t *testing.T) {
	got := review.BuildReviewBackground("English", "My PR", "Intent", "focus on tests")
	if !strings.HasPrefix(got, "### PR Description Context") {
		t.Fatalf("PR section first: %q", got)
	}
	if !strings.Contains(got, "**Title:** My PR") {
		t.Fatalf("title: %q", got)
	}
	if !strings.Contains(got, "**Body:**\nIntent") {
		t.Fatalf("body: %q", got)
	}
	idxPR := strings.Index(got, "### PR Description Context")
	idxReq := strings.Index(got, "### Requirements")
	if idxPR < 0 || idxReq < 0 || idxPR >= idxReq {
		t.Fatalf("order: %q", got)
	}
	if !strings.Contains(got, "focus on tests") || !strings.Contains(got, "Every comment must set path") {
		t.Fatalf("requirements: %q", got)
	}
}

func TestBuildReviewBackgroundTitleOnly(t *testing.T) {
	got := review.BuildReviewBackground("Japanese", "タイトルのみ", "", "")
	if !strings.Contains(got, "**タイトル:** タイトルのみ") {
		t.Fatalf("title only: %q", got)
	}
	if strings.Contains(got, "**本文:**") {
		t.Fatalf("body label should be omitted: %q", got)
	}
}

func TestBuildReviewBackgroundEmptyBoth(t *testing.T) {
	got := review.BuildReviewBackground("English", "", "  ", "repo req")
	if strings.Contains(got, "PR Description Context") {
		t.Fatalf("PR section omitted: %q", got)
	}
	if !strings.HasPrefix(got, "### Requirements") {
		t.Fatalf("requirements only: %q", got)
	}
}

func TestBuildReviewBackgroundTruncation(t *testing.T) {
	body := strings.Repeat("あ", 8001)
	got := review.BuildReviewBackground("English", "T", body, "")
	if strings.Count(got, "あ") != 8000 {
		t.Fatalf("rune count: %d", strings.Count(got, "あ"))
	}
	if !strings.Contains(got, "truncated at 8,000 runes") {
		t.Fatalf("marker: %q", got[len(got)-80:])
	}
}

func TestBuildReviewBackgroundChineseLabels(t *testing.T) {
	got := review.BuildReviewBackground("Chinese", "标题", "正文", "")
	if !strings.Contains(got, "### PR 描述上下文") || !strings.Contains(got, "**标题:** 标题") {
		t.Fatalf("chinese labels: %q", got)
	}
}
