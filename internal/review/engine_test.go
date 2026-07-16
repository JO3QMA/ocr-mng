package review_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestScheduleReview_manualClosedPR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state": "closed", "number": 5, "title": "", "body": "", "base": {"ref": "main"}, "head": {"sha": "x"}, "labels": []}`))
	}))
	defer srv.Close()

	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: srv.URL, WebBaseURL: "https://github.com",
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

	e := review.NewEngine(config.Config{DataDir: t.TempDir()}, st, slog.Default())
	err = e.ScheduleReview(ctx, review.ScheduleRequest{
		RepoID: repoID, PRNumber: 5, TriggerKind: "manual",
	})
	if err == nil {
		t.Fatal("expected error scheduling closed PR")
	}
}

func TestTryDispatch_drainsPendingWhenSlotFrees(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/repos/acme/app/pulls/1":
			body = `{"state":"open","number":1,"title":"one","body":"","base":{"ref":"main"},"head":{"sha":"aaa"},"labels":[]}`
		case "/repos/acme/app/pulls/2":
			body = `{"state":"open","number":2,"title":"two","body":"","base":{"ref":"main"},"head":{"sha":"bbb"},"labels":[]}`
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: srv.URL, WebBaseURL: "https://github.com",
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
	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gs.MaxConcurrentReviews = 1
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}

	e := review.NewEngine(config.Config{DataDir: t.TempDir()}, st, slog.Default())
	req := func(pr int, sha string) review.ScheduleRequest {
		return review.ScheduleRequest{
			RepoID: repoID, PRNumber: pr, HeadSHA: sha, BaseRef: "main", TriggerKind: "label",
		}
	}
	if err := e.ScheduleReview(ctx, req(1, "aaa")); err != nil {
		t.Fatal(err)
	}
	if err := e.ScheduleReview(ctx, req(2, "bbb")); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := st.ListReviewRuns(ctx, repoID, 10)
		if err != nil {
			t.Fatal(err)
		}
		byPR := map[int]store.ReviewRun{}
		for _, r := range runs {
			byPR[r.PRNumber] = r
		}
		r1, ok1 := byPR[1]
		r2, ok2 := byPR[2]
		if !ok1 || !ok2 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if r1.Status == "pending" && r2.Status == "pending" {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if r1.Status != "pending" && r2.Status == "pending" {
			// First run claimed the only slot; second is still queued.
			break
		}
		if r1.Status != "pending" && r2.Status != "pending" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := st.ListReviewRuns(ctx, repoID, 10)
		if err != nil {
			t.Fatal(err)
		}
		done := 0
		pending := 0
		for _, r := range runs {
			if r.PRNumber != 1 && r.PRNumber != 2 {
				continue
			}
			switch r.Status {
			case "pending":
				pending++
			case "success", "failed":
				done++
			default:
				// running — keep waiting
			}
		}
		if done == 2 && pending == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for queued run to finish after slot freed")
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
