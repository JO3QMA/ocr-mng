package store_test

import (
	"context"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
)

func providerModelIDByName(t *testing.T, st *store.Store, ctx context.Context, pid int64, name string) int64 {
	t.Helper()
	models, err := st.ListLLMProviderModels(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if m.ModelName == name {
			return m.ID
		}
	}
	t.Fatalf("model %q not found in %#v", name, models)
	return 0
}

func TestSyncLLMProviderModels(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P", ProviderKey: "openai", Kind: "custom",
		APIBaseURL: "https://example/v1", Protocol: "openai", Enabled: true,
	}, "sk")
	if err != nil {
		t.Fatal(err)
	}

	manualID, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: "gpt-4", Enabled: true, Source: store.ModelSourceManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SyncLLMProviderModels(ctx, pid, []string{"old-api"}); err != nil {
		t.Fatal(err)
	}
	apiID := providerModelIDByName(t, st, ctx, pid, "old-api")

	result, err := st.SyncLLMProviderModels(ctx, pid, []string{"gpt-4", "gpt-3.5-turbo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Deleted != 1 || result.New != 1 {
		t.Fatalf("got %#v", result)
	}

	promoted, err := st.GetLLMProviderModel(ctx, manualID)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Source != store.ModelSourceAPI || !promoted.Enabled {
		t.Fatalf("promoted: %#v", promoted)
	}

	newModel, err := st.ListLLMProviderModels(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	var foundNew bool
	for _, m := range newModel {
		if m.ModelName == "gpt-3.5-turbo" {
			foundNew = true
			if m.Source != store.ModelSourceAPI || m.Enabled || !m.IsNew {
				t.Fatalf("new row: %#v", m)
			}
		}
		if m.ModelName == "old-api" {
			t.Fatal("removed api model should be deleted")
		}
	}
	if !foundNew {
		t.Fatal("expected gpt-3.5-turbo")
	}

	if _, err := st.GetLLMProviderModel(ctx, apiID); err == nil {
		t.Fatal("old-api should be gone")
	}
}

func TestSyncLLMProviderModelsPrunesGlobalRotation(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P", ProviderKey: "openai", Kind: "custom",
		APIBaseURL: "https://example/v1", Protocol: "openai", Enabled: true,
	}, "sk")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SyncLLMProviderModels(ctx, pid, []string{"gone"}); err != nil {
		t.Fatal(err)
	}
	mid := providerModelIDByName(t, st, ctx, pid, "gone")
	gone, err := st.GetLLMProviderModel(ctx, mid)
	if err != nil {
		t.Fatal(err)
	}
	gone.Enabled = true
	if err := st.UpdateLLMProviderModel(ctx, gone); err != nil {
		t.Fatal(err)
	}

	gs, err := st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gs.DefaultLLMRotation = []store.LLMPair{{ProviderID: pid, ModelID: mid}}
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}

	if _, err := st.SyncLLMProviderModels(ctx, pid, []string{}); err != nil {
		t.Fatal(err)
	}

	gs, err = st.GetGlobalSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.DefaultLLMRotation) != 0 {
		t.Fatalf("expected empty rotation, got %#v", gs.DefaultLLMRotation)
	}
}

func TestDeleteLLMProviderModelRejectsAPI(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
	}, "sk")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SyncLLMProviderModels(ctx, pid, []string{"m1"}); err != nil {
		t.Fatal(err)
	}
	mid := providerModelIDByName(t, st, ctx, pid, "m1")
	if err := st.DeleteLLMProviderModel(ctx, mid); err == nil {
		t.Fatal("expected reject")
	}
}
