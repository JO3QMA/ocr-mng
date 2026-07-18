package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
)

func openLLMStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func mustCreateLLMPair(t *testing.T, st *store.Store, ctx context.Context, name, key string) (providerID, modelID int64) {
	t.Helper()
	providerID, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: name, ProviderKey: key, Kind: "builtin", Enabled: true,
	}, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	modelID, err = st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: providerID, ModelName: "model-a", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return providerID, modelID
}

func TestLLMProviderCustomRequiresURLAndProtocol(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()

	_, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "gw", ProviderKey: "my-gw", Kind: "custom", Enabled: true,
	}, "sk")
	if err == nil || !strings.Contains(err.Error(), "api_base_url") {
		t.Fatalf("expected custom require url: %v", err)
	}

	id, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "gw", ProviderKey: "my-gw", Kind: "custom",
		APIBaseURL: "https://example/v1", Enabled: true,
	}, "sk")
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.GetLLMProvider(ctx, id)
	if err != nil || p.Protocol != "openai" {
		t.Fatalf("expected inferred openai: %+v err=%v", p, err)
	}

	_, err = st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "bad", ProviderKey: "b", Kind: "builtin", Protocol: "nope", Enabled: true,
	}, "sk")
	if err == nil || !strings.Contains(err.Error(), "invalid protocol") {
		t.Fatalf("expected protocol enum: %v", err)
	}
}

func TestLLMProviderAPIKeySetClearKeep(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()

	id, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "anthropic", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
	}, "sk-1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.GetLLMProvider(ctx, id)
	if err != nil || !p.HasAPIKey {
		t.Fatalf("provider: %+v err=%v", p, err)
	}
	got, err := st.LLMProviderAPIKey(ctx, id)
	if err != nil || got != "sk-1" {
		t.Fatalf("key=%q err=%v", got, err)
	}

	if err := st.UpdateLLMProvider(ctx, p, "", false); err != nil {
		t.Fatal(err)
	}
	p, _ = st.GetLLMProvider(ctx, id)
	if !p.HasAPIKey {
		t.Fatal("expected key kept")
	}

	if err := st.UpdateLLMProvider(ctx, p, "sk-2", false); err != nil {
		t.Fatal(err)
	}
	got, err = st.LLMProviderAPIKey(ctx, id)
	if err != nil || got != "sk-2" {
		t.Fatalf("set key=%q err=%v", got, err)
	}

	if err := st.UpdateLLMProvider(ctx, p, "", true); err != nil {
		t.Fatal(err)
	}
	p, _ = st.GetLLMProvider(ctx, id)
	if p.HasAPIKey {
		t.Fatal("expected key cleared")
	}
	got, err = st.LLMProviderAPIKey(ctx, id)
	if err != nil || got != "" {
		t.Fatalf("cleared key=%q err=%v", got, err)
	}
}

func TestSaveLLMPair_partialRejected(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	providerID, _ := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")

	err := store.ValidateLLMPairIDs(providerID, 0)
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected partial reject: %v", err)
	}

	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gs.DefaultLLMProviderID = providerID
	gs.DefaultLLMModelID = 0
	if err := st.SaveGlobalSettings(ctx, gs); err == nil {
		t.Fatal("expected save reject")
	}
}

func TestGlobalLLMPair_clearRejected(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	providerID, modelID := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")

	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gs.DefaultLLMProviderID = providerID
	gs.DefaultLLMModelID = modelID
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}

	gs.DefaultLLMProviderID = 0
	gs.DefaultLLMModelID = 0
	err = st.SaveGlobalSettings(ctx, gs)
	if err == nil || !strings.Contains(err.Error(), "cannot be cleared") {
		t.Fatalf("expected clear reject: %v", err)
	}
}

func TestDeleteProvider_inUse(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	providerID, modelID := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")

	gs, _ := st.GetGlobalSettings(ctx)
	gs.DefaultLLMProviderID = providerID
	gs.DefaultLLMModelID = modelID
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}

	err := st.DeleteLLMProvider(ctx, providerID)
	if err == nil || !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("expected in-use reject: %v", err)
	}
	err = st.DeleteLLMProviderModel(ctx, modelID)
	if err == nil || !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("expected model in-use reject: %v", err)
	}
}

func TestResolveLLM_disabledModel(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	providerID, modelID := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")

	m, err := st.GetLLMProviderModel(ctx, modelID)
	if err != nil {
		t.Fatal(err)
	}
	m.Enabled = false
	if err := st.UpdateLLMProviderModel(ctx, m); err != nil {
		t.Fatal(err)
	}

	gs, _ := st.GetGlobalSettings(ctx)
	gs.DefaultLLMProviderID = providerID
	gs.DefaultLLMModelID = modelID
	err = st.SaveGlobalSettings(ctx, gs)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled reject: %v", err)
	}
}

func TestRepoLLMPair_overrideAndClear(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	providerID, modelID := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	provider2, model2 := mustCreateLLMPair(t, st, ctx, "p2", "openai")

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
		LLMProviderID: providerID, LLMModelID: modelID,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	rv, err := st.GetRepo(ctx, repoID)
	if err != nil || rv.LLMProviderID != providerID || rv.LLMModelID != modelID {
		t.Fatalf("repo pair: %+v err=%v", rv, err)
	}

	rv.LLMProviderID = provider2
	rv.LLMModelID = model2
	if err := st.UpdateRepo(ctx, rv.Repo, "", false); err != nil {
		t.Fatal(err)
	}
	rv, _ = st.GetRepo(ctx, repoID)
	if rv.LLMProviderID != provider2 || rv.LLMModelID != model2 {
		t.Fatalf("override: %+v", rv)
	}

	rv.LLMProviderID = 0
	rv.LLMModelID = 0
	if err := st.UpdateRepo(ctx, rv.Repo, "", false); err != nil {
		t.Fatal(err)
	}
	rv, _ = st.GetRepo(ctx, repoID)
	if rv.LLMProviderID != 0 || rv.LLMModelID != 0 {
		t.Fatalf("cleared: %+v", rv)
	}
}

func TestDeleteProvider_afterRepoCleared(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	providerID, modelID := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")

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
		LLMProviderID: providerID, LLMModelID: modelID,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteLLMProvider(ctx, providerID); err == nil {
		t.Fatal("expected referenced by repo")
	}

	rv, _ := st.GetRepo(ctx, repoID)
	rv.LLMProviderID, rv.LLMModelID = 0, 0
	if err := st.UpdateRepo(ctx, rv.Repo, "", false); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteLLMProviderModel(ctx, modelID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteLLMProvider(ctx, providerID); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLLMPairIDs(t *testing.T) {
	if err := store.ValidateLLMPairIDs(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateLLMPairIDs(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateLLMPairIDs(1, 0); err == nil {
		t.Fatal("expected error for provider-only")
	}
	if err := store.ValidateLLMPairIDs(0, 1); err == nil {
		t.Fatal("expected error for model-only")
	}
}
