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
