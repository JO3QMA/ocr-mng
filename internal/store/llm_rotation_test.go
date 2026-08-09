package store_test

import (
	"context"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestClaimLLMRotation_roundRobinAndSkip(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	p1, m1 := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	p2, m2 := mustCreateLLMPair(t, st, ctx, "p2", "openai")
	pairs := []store.LLMPair{
		{ProviderID: p1, ModelID: m1},
		{ProviderID: p2, ModelID: m2},
	}
	gs, _ := st.GetGlobalSettings(ctx)
	gs.DefaultLLMRotation = pairs
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}

	a, err := st.ClaimLLMRotation(ctx, store.LLMRotationKeyGlobal, pairs)
	if err != nil || a.ModelID != m1 {
		t.Fatalf("first: %+v %v", a, err)
	}
	b, err := st.ClaimLLMRotation(ctx, store.LLMRotationKeyGlobal, pairs)
	if err != nil || b.ModelID != m2 {
		t.Fatalf("second: %+v %v", b, err)
	}
	c, err := st.ClaimLLMRotation(ctx, store.LLMRotationKeyGlobal, pairs)
	if err != nil || c.ModelID != m1 {
		t.Fatalf("third: %+v %v", c, err)
	}

	mod, _ := st.GetLLMProviderModel(ctx, m1)
	mod.Enabled = false
	if err := st.UpdateLLMProviderModel(ctx, mod); err != nil {
		t.Fatal(err)
	}
	d, err := st.ClaimLLMRotation(ctx, store.LLMRotationKeyGlobal, pairs)
	if err != nil || d.ModelID != m2 {
		t.Fatalf("skip disabled: %+v %v", d, err)
	}
}

func TestClaimLLMRotation_resetOnSetChange(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	p1, m1 := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	p2, m2 := mustCreateLLMPair(t, st, ctx, "p2", "openai")
	gs, _ := st.GetGlobalSettings(ctx)
	gs.DefaultLLMRotation = []store.LLMPair{{ProviderID: p1, ModelID: m1}, {ProviderID: p2, ModelID: m2}}
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}
	_, _ = st.ClaimLLMRotation(ctx, store.LLMRotationKeyGlobal, gs.EffectiveLLMRotation())

	gs.DefaultLLMRotation = []store.LLMPair{{ProviderID: p2, ModelID: m2}, {ProviderID: p1, ModelID: m1}}
	if err := st.SaveGlobalSettings(ctx, gs); err != nil {
		t.Fatal(err)
	}
	got, err := st.ClaimLLMRotation(ctx, store.LLMRotationKeyGlobal, gs.EffectiveLLMRotation())
	if err != nil || got.ModelID != m2 {
		t.Fatalf("after reorder reset to first: %+v %v", got, err)
	}
}

func TestValidateLLMRotation_duplicate(t *testing.T) {
	err := store.ValidateLLMRotation([]store.LLMPair{
		{ProviderID: 1, ModelID: 2},
		{ProviderID: 1, ModelID: 2},
	})
	if err == nil {
		t.Fatal("expected duplicate reject")
	}
}
