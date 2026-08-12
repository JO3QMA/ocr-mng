package review_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
)

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

func TestResolveLLM_roundRobin(t *testing.T) {
	st := openReviewStore(t)
	ctx := context.Background()
	pid1, mid1 := mustLLMPair(t, st, ctx, "P1", "anthropic", "m1")
	pid2, mid2 := mustLLMPair(t, st, ctx, "P2", "openai", "m2")
	gs := saveGlobalRotation(t, st, ctx,
		store.LLMPair{ProviderID: pid1, ModelID: mid1},
		store.LLMPair{ProviderID: pid2, ModelID: mid2},
	)

	a, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err != nil || a.ModelName != "m1" {
		t.Fatalf("1st: %+v %v", a, err)
	}
	b, err := review.ResolveLLMSelection(ctx, st, gs, store.RepoView{}, "Japanese")
	if err != nil || b.ModelName != "m2" {
		t.Fatalf("2nd: %+v %v", b, err)
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
