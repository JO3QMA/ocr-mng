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
	n, err := st.FailInterruptedReviewRuns(ctx, "interrupted")
	if err != nil || n != 1 {
		t.Fatalf("fail interrupted: n=%d err=%v", n, err)
	}
	run, err := st.ListReviewRuns(ctx, repoID, 1)
	if err != nil || len(run) != 1 {
		t.Fatalf("runs: %+v err=%v", run, err)
	}
	if run[0].Status != "failed" || run[0].ErrorMessage != "interrupted" || run[0].FinishedAt == nil {
		t.Fatalf("run: %+v", run[0])
	}
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
