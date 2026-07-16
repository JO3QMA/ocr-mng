package review_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/config"
	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestNewEngine(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	e := review.NewEngine(config.Config{}, st, slog.Default())
	if e == nil {
		t.Fatal("nil engine")
	}
	gs, err := st.GetGlobalSettings(context.Background())
	if err != nil || gs.MaxConcurrentReviews < 1 {
		t.Fatalf("settings: %+v err=%v", gs, err)
	}
}

func TestScheduleReview_createsPending(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustRepo(t, st, ctx)

	e := review.NewEngine(config.Config{DataDir: t.TempDir()}, st, slog.Default())
	if err := e.ScheduleReview(ctx, review.ScheduleRequest{
		RepoID: repoID, PRNumber: 3, HeadSHA: "sha", BaseRef: "main", TriggerKind: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	runs, err := st.ListReviewRuns(ctx, repoID, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs: %+v err=%v", runs, err)
	}
	if runs[0].Status != "pending" && runs[0].Status != "running" && runs[0].Status != "failed" {
		t.Fatalf("status: %s", runs[0].Status)
	}
	// Without git host reachable, dispatch may fail the run; pending must exist first or move to failed.
	if runs[0].Status == "pending" {
		return
	}
}

func TestScheduleReview_duplicate_returnsNil(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustRepo(t, st, ctx)

	e := review.NewEngine(config.Config{DataDir: t.TempDir()}, st, slog.Default())
	req := review.ScheduleRequest{
		RepoID: repoID, PRNumber: 9, HeadSHA: "sha", BaseRef: "main", TriggerKind: "manual",
	}
	if err := e.ScheduleReview(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := e.ScheduleReview(ctx, req); err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	runs, err := st.ListReviewRuns(ctx, repoID, 10)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, r := range runs {
		if r.PRNumber == 9 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected 1 run for PR 9, got %d", n)
	}
}

func mustRepo(t *testing.T, st *store.Store, ctx context.Context) int64 {
	t.Helper()
	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "pat")
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := st.CreateRepo(ctx, store.Repo{
		GitHostID: hostID, Owner: "acme", Name: "app",
		DefaultBranch: "main", TriggerLabel: "review", CommentMode: "inline", Enabled: true,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	return repoID
}
