package store_test

import (
	"context"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestOpenAndGlobalSettings(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if gs.PollIntervalSeconds < 1 || gs.MinPollIntervalSeconds < 1 {
		t.Fatalf("unexpected defaults: %+v", gs)
	}
	if gs.UILanguage != "ja" || gs.ReviewLanguage != "Japanese" {
		t.Fatalf("language defaults: %+v", gs)
	}

	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "host-pat")
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := st.CreateRepo(ctx, store.Repo{
		GitHostID: hostID, Owner: "acme", Name: "app",
		DefaultBranch: "main", TriggerLabel: "review", CommentMode: "inline", Enabled: true,
	}, "repo-pat")
	if err != nil {
		t.Fatal(err)
	}
	repos, err := st.ListRepos(ctx)
	if err != nil || len(repos) != 1 || repos[0].ID != repoID {
		t.Fatalf("repos: %+v err=%v", repos, err)
	}
	if _, err := st.RepoPAT(ctx, repoID); err != nil {
		t.Fatal(err)
	}
}

func TestFailInterruptedReviewRuns(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "")
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

	if _, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 1, HeadSHA: "abc", BaseRef: "main",
		Status: "running", TriggerKind: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	pendingID, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 2, HeadSHA: "def", BaseRef: "main",
		Status: "pending", TriggerKind: "label",
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := st.FailInterruptedReviewRuns(ctx, "interrupted")
	if err != nil || n != 1 {
		t.Fatalf("fail interrupted: n=%d err=%v", n, err)
	}
	run, err := st.ListReviewRuns(ctx, repoID, 10)
	if err != nil || len(run) != 2 {
		t.Fatalf("runs: %+v err=%v", run, err)
	}
	byPR := map[int]store.ReviewRun{}
	for _, r := range run {
		byPR[r.PRNumber] = r
	}
	if byPR[1].Status != "failed" || byPR[1].ErrorMessage != "interrupted" {
		t.Fatalf("running run: %+v", byPR[1])
	}
	if byPR[2].Status != "pending" || byPR[2].ID != pendingID {
		t.Fatalf("pending run: %+v", byPR[2])
	}
}

func TestHasActiveReviewRun(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustTestRepo(t, st, ctx)

	active, err := st.HasActiveReviewRun(ctx, repoID, 7)
	if err != nil || active {
		t.Fatalf("initial: active=%v err=%v", active, err)
	}
	if _, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 7, HeadSHA: "a", BaseRef: "main",
		Status: "pending", TriggerKind: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	active, err = st.HasActiveReviewRun(ctx, repoID, 7)
	if err != nil || !active {
		t.Fatalf("pending: active=%v err=%v", active, err)
	}
}

func TestCreatePendingReviewRunIfAbsent(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustTestRepo(t, st, ctx)

	run := store.ReviewRun{
		RepoID: repoID, PRNumber: 11, HeadSHA: "sha", BaseRef: "main",
		TriggerKind: "manual",
	}
	id, created, err := st.CreatePendingReviewRunIfAbsent(ctx, run)
	if err != nil || !created || id <= 0 {
		t.Fatalf("first insert: id=%d created=%v err=%v", id, created, err)
	}
	_, created, err = st.CreatePendingReviewRunIfAbsent(ctx, run)
	if err != nil || created {
		t.Fatalf("duplicate: created=%v err=%v", created, err)
	}
}

func TestCreatePendingReviewRunIfAbsent_concurrent(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustTestRepo(t, st, ctx)

	run := store.ReviewRun{
		RepoID: repoID, PRNumber: 12, HeadSHA: "sha", BaseRef: "main",
		TriggerKind: "manual",
	}
	const n = 8
	created := make(chan bool, n)
	for range n {
		go func() {
			_, ok, err := st.CreatePendingReviewRunIfAbsent(ctx, run)
			created <- err == nil && ok
		}()
	}
	var count int
	for range n {
		if <-created {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 created run, got %d", count)
	}
}

func TestClaimNextPendingReviewRun_skipsBusyRepo(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustTestRepo(t, st, ctx)

	if _, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 1, HeadSHA: "a", BaseRef: "main",
		Status: "running", TriggerKind: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	oldID, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 2, HeadSHA: "b", BaseRef: "main",
		Status: "pending", TriggerKind: "label",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherRepo := mustTestRepoName(t, st, ctx, "other")

	newID, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: otherRepo, PRNumber: 1, HeadSHA: "c", BaseRef: "main",
		Status: "pending", TriggerKind: "label",
	})
	if err != nil {
		t.Fatal(err)
	}

	claimed, ok, err := st.ClaimNextPendingReviewRun(ctx)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	if claimed.ID != newID || claimed.RepoID != otherRepo {
		t.Fatalf("expected other repo pending, got %+v", claimed)
	}
	still, ok, err := st.ClaimNextPendingReviewRun(ctx)
	if err != nil || ok {
		t.Fatalf("busy repo skip: ok=%v run=%+v err=%v", ok, still, err)
	}
	pending, err := st.GetReviewRun(ctx, oldID)
	if err != nil || pending.Status != "pending" {
		t.Fatalf("old pending: %+v err=%v", pending, err)
	}
}

func TestListReviewRunsIncludesRepoOwnerName(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustTestRepo(t, st, ctx)

	if _, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 3, HeadSHA: "abc", BaseRef: "main",
		Status: "pending", TriggerKind: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	runs, err := st.ListReviewRuns(ctx, 0, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs: %+v err=%v", runs, err)
	}
	got := runs[0]
	if got.RepoOwner != "acme" || got.RepoName != "app" {
		t.Fatalf("owner/name: %+v", got)
	}
	if got.RepoDisplay() != "acme/app" {
		t.Fatalf("display: %q", got.RepoDisplay())
	}
}

func mustTestRepo(t *testing.T, st *store.Store, ctx context.Context) int64 {
	t.Helper()
	return mustTestRepoName(t, st, ctx, "app")
}

func mustTestRepoName(t *testing.T, st *store.Store, ctx context.Context, name string) int64 {
	t.Helper()
	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github-" + name, Kind: "github",
		APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := st.CreateRepo(ctx, store.Repo{
		GitHostID: hostID, Owner: "acme", Name: name,
		DefaultBranch: "main", TriggerLabel: "review", CommentMode: "inline", Enabled: true,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	return repoID
}

func TestUpdatePATSetClearKeep(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	hostID, err := st.CreateGitHost(ctx, store.GitHost{
		Name: "github", Kind: "github",
		APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "host-pat-1")
	if err != nil {
		t.Fatal(err)
	}
	h, err := st.GetGitHost(ctx, hostID)
	if err != nil || !h.HasHostPAT {
		t.Fatalf("host: %+v err=%v", h, err)
	}
	// keep
	if err := st.UpdateGitHost(ctx, h, "", false); err != nil {
		t.Fatal(err)
	}
	h, _ = st.GetGitHost(ctx, hostID)
	if !h.HasHostPAT {
		t.Fatal("expected PAT kept")
	}
	// set
	if err := st.UpdateGitHost(ctx, h, "host-pat-2", false); err != nil {
		t.Fatal(err)
	}
	h, _ = st.GetGitHost(ctx, hostID)
	if !h.HasHostPAT {
		t.Fatal("expected PAT set")
	}
	// clear
	if err := st.UpdateGitHost(ctx, h, "", true); err != nil {
		t.Fatal(err)
	}
	h, _ = st.GetGitHost(ctx, hostID)
	if h.HasHostPAT {
		t.Fatal("expected PAT cleared")
	}

	repoID, err := st.CreateRepo(ctx, store.Repo{
		GitHostID: hostID, Owner: "acme", Name: "app",
		DefaultBranch: "main", TriggerLabel: "review", CommentMode: "inline", Enabled: true,
	}, "repo-pat-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.RepoPAT(ctx, repoID); err != nil {
		t.Fatal(err)
	}
	rv, err := st.GetRepo(ctx, repoID)
	if err != nil {
		t.Fatal(err)
	}
	// keep (falls back to missing host PAT → error unless we set host again)
	if err := st.UpdateGitHost(ctx, h, "host-again", false); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateRepo(ctx, rv.Repo, "", false); err != nil {
		t.Fatal(err)
	}
	if got, err := st.RepoPAT(ctx, repoID); err != nil || got != "repo-pat-1" {
		t.Fatalf("keep repo pat: got=%q err=%v", got, err)
	}
	if err := st.UpdateRepo(ctx, rv.Repo, "repo-pat-2", false); err != nil {
		t.Fatal(err)
	}
	if got, err := st.RepoPAT(ctx, repoID); err != nil || got != "repo-pat-2" {
		t.Fatalf("set repo pat: got=%q err=%v", got, err)
	}
	if err := st.UpdateRepo(ctx, rv.Repo, "", true); err != nil {
		t.Fatal(err)
	}
	if got, err := st.RepoPAT(ctx, repoID); err != nil || got != "host-again" {
		t.Fatalf("clear repo pat falls back to host: got=%q err=%v", got, err)
	}
}
