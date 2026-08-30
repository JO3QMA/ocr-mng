package review_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	dataDir := t.TempDir()
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustRepo(t, st, ctx)

	e := review.NewEngine(config.Config{DataDir: dataDir}, st, slog.Default())
	t.Cleanup(func() { waitReviewEngineIdle(t, e, st, ctx, repoID) })
	if err := e.ScheduleReview(ctx, review.ScheduleRequest{
		RepoID: repoID, PRNumber: 3, HeadSHA: "sha", BaseRef: "main", TriggerKind: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	runs, err := st.ListReviewRuns(ctx, repoID, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs: %+v err=%v", runs, err)
	}
	switch runs[0].Status {
	case "pending", "running", "failed":
	default:
		t.Fatalf("status: %s", runs[0].Status)
	}
}

func waitReviewEngineIdle(t *testing.T, e *review.Engine, st *store.Store, ctx context.Context, repoID int64) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if e.RunningReviews() > 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		runs, err := st.ListReviewRuns(ctx, repoID, 50)
		if err != nil {
			return
		}
		active := false
		for _, r := range runs {
			if r.Status == "pending" || r.Status == "running" {
				active = true
				break
			}
		}
		if !active {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("timed out waiting for review engine idle")
}

func TestScheduleReview_duplicate_returnsNil(t *testing.T) {
	dataDir := t.TempDir()
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	repoID := mustRepo(t, st, ctx)

	e := review.NewEngine(config.Config{DataDir: dataDir}, st, slog.Default())
	t.Cleanup(func() { waitReviewEngineIdle(t, e, st, ctx, repoID) })
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
	providerID, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "anthropic", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
	}, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	modelID, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: providerID, ModelName: "claude-x", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gs.MaxConcurrentReviews = 1
	gs.DefaultLLMRotation = []store.LLMPair{{ProviderID: providerID, ModelID: modelID}}
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}

	e := review.NewEngine(config.Config{DataDir: t.TempDir()}, st, slog.Default())
	t.Cleanup(func() { waitReviewEngineIdle(t, e, st, ctx, repoID) })
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

func openReviewStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func mustLLMPair(t *testing.T, st *store.Store, ctx context.Context, name, key, model string) (int64, int64) {
	t.Helper()
	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: name, ProviderKey: key, Kind: "builtin", Enabled: true,
	}, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	mid, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: model, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return pid, mid
}

func saveGlobalRotation(t *testing.T, st *store.Store, ctx context.Context, pairs ...store.LLMPair) store.GlobalSettings {
	t.Helper()
	gs, _ := st.GetGlobalSettings(ctx)
	gs.DefaultLLMRotation = pairs
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}
	gs, _ = st.GetGlobalSettings(ctx)
	return gs
}

func TestResolveLLM_emptyGlobalRotationFails(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	gs, _ := st.GetGlobalSettings(ctx)
	_, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty rotation error: %v", err)
	}
}

func TestResolveLLM_ledgerPair(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid, mid := mustLLMPair(t, st, ctx, "Anthropic", "anthropic", "claude-x")
	gs := saveGlobalRotation(t, st, ctx, store.LLMPair{ProviderID: pid, ModelID: mid})

	sel, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err != nil {
		t.Fatal(err)
	}
	if sel.ProviderName != "Anthropic" || sel.ModelName != "claude-x" {
		t.Fatalf("%+v", sel)
	}
	if sel.ProviderFlag != "anthropic" || sel.ModelFlag != "claude-x" {
		t.Fatalf("flags: %+v", sel)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(sel.ConfigJSON), &m)
	if m["provider"] != "anthropic" {
		t.Fatalf("config: %s", sel.ConfigJSON)
	}
	providers := m["providers"].(map[string]any)
	entry := providers["anthropic"].(map[string]any)
	if entry["api_key"] != "sk-test" {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestResolveLLM_repoOverride(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid1, mid1 := mustLLMPair(t, st, ctx, "P1", "anthropic", "m1")
	pid2, mid2 := mustLLMPair(t, st, ctx, "P2", "openai", "m2")
	gs := saveGlobalRotation(t, st, ctx, store.LLMPair{ProviderID: pid1, ModelID: mid1})

	sel, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{
		Repo: store.Repo{LLMRotation: []store.LLMPair{{ProviderID: pid2, ModelID: mid2}}},
	}, "English")
	if err != nil {
		t.Fatal(err)
	}
	if sel.ProviderName != "P2" || sel.ModelName != "m2" {
		t.Fatalf("%+v", sel)
	}
	if sel.ProviderFlag != "openai" || sel.ModelFlag != "m2" {
		t.Fatalf("flags: %+v", sel)
	}
}

func TestResolveLLM_repoCleared(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid, mid := mustLLMPair(t, st, ctx, "P1", "anthropic", "m1")
	gs := saveGlobalRotation(t, st, ctx, store.LLMPair{ProviderID: pid, ModelID: mid})

	sel, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err != nil || sel.ProviderName != "P1" {
		t.Fatalf("%+v %v", sel, err)
	}
}

func TestResolveLLM_disabledModel(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid, mid := mustLLMPair(t, st, ctx, "P1", "anthropic", "m1")
	m, _ := st.GetLLMProviderModel(ctx, mid)
	m.Enabled = false
	_ = st.UpdateLLMProviderModel(ctx, m)

	mid2, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: "m2", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	gs := saveGlobalRotation(t, st, ctx, store.LLMPair{ProviderID: pid, ModelID: mid2})

	_, err = review.ResolveLLMSelection(ctx, st, gs, store.RepoView{
		Repo: store.Repo{LLMRotation: []store.LLMPair{{ProviderID: pid, ModelID: mid}}},
	}, "Japanese")
	if err == nil || !strings.Contains(err.Error(), "no usable") {
		t.Fatalf("expected no usable: %v", err)
	}
}

func TestResolveLLM_picksFromUsable(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid1, mid1 := mustLLMPair(t, st, ctx, "P1", "anthropic", "m1")
	pid2, mid2 := mustLLMPair(t, st, ctx, "P2", "openai", "m2")
	gs := saveGlobalRotation(t, st, ctx,
		store.LLMPair{ProviderID: pid1, ModelID: mid1},
		store.LLMPair{ProviderID: pid2, ModelID: mid2},
	)

	got, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelName != "m1" && got.ModelName != "m2" {
		t.Fatalf("picked unknown: %+v", got)
	}
}

func TestResolveLLM_missingAPIKey(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P1", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	mid, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: "m1", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	gs := saveGlobalRotation(t, st, ctx, store.LLMPair{ProviderID: pid, ModelID: mid})

	_, err = review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err == nil || (!strings.Contains(err.Error(), "no api key") && !strings.Contains(err.Error(), "no usable")) {
		t.Fatalf("expected missing key: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "sk-") {
		t.Fatal("error must not contain secrets")
	}
}

func TestOCRHome_perRunIsolation(t *testing.T) {
	a := review.OCRHomeDir("/data", 1)
	b := review.OCRHomeDir("/data", 2)
	if a == b || !strings.Contains(a, "run-1") || !strings.Contains(b, "run-2") {
		t.Fatalf("%s vs %s", a, b)
	}
	root := t.TempDir()
	orphan := filepath.Join(root, "run-99")
	keep := filepath.Join(root, "other")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	review.PruneOrphanOCRHomes(root, nil)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan should be removed: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("non-run dir should remain")
	}
}

func TestReviewRun_llmSnapshotColumns(t *testing.T) {
	st := openReviewStore(t)
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
	id, err := st.CreateReviewRun(ctx, store.ReviewRun{
		RepoID: repoID, PRNumber: 1, HeadSHA: "abc", BaseRef: "main",
		Status: "running", TriggerKind: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.GetReviewRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	run.Status = "failed"
	run.ErrorMessage = "llm: missing"
	run.LLMProviderName = "Anthropic"
	run.LLMModelName = "claude-x"
	if err := st.UpdateReviewRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetReviewRun(ctx, id)
	if err != nil || got.LLMProviderName != "Anthropic" || got.LLMModelName != "claude-x" {
		t.Fatalf("%+v err=%v", got, err)
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
