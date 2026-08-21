package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestPickLLMRotation_singleton(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	p1, m1 := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	pairs := []store.LLMPair{{ProviderID: p1, ModelID: m1}}

	got, err := st.PickLLMRotation(ctx, pairs)
	if err != nil || got.ModelID != m1 {
		t.Fatalf("singleton: %+v %v", got, err)
	}
}

func TestPickLLMRotation_skipsUnusable(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	p1, m1 := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	p2, m2 := mustCreateLLMPair(t, st, ctx, "p2", "openai")
	pairs := []store.LLMPair{
		{ProviderID: p1, ModelID: m1},
		{ProviderID: p2, ModelID: m2},
	}

	mod, _ := st.GetLLMProviderModel(ctx, m1)
	mod.Enabled = false
	if err := st.UpdateLLMProviderModel(ctx, mod); err != nil {
		t.Fatal(err)
	}
	got, err := st.PickLLMRotation(ctx, pairs)
	if err != nil || got.ModelID != m2 {
		t.Fatalf("skip disabled: %+v %v", got, err)
	}
}

func TestPickLLMRotation_allUnusable(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	p1, m1 := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	p2, m2 := mustCreateLLMPair(t, st, ctx, "p2", "openai")
	pairs := []store.LLMPair{
		{ProviderID: p1, ModelID: m1},
		{ProviderID: p2, ModelID: m2},
	}
	for _, id := range []int64{m1, m2} {
		mod, _ := st.GetLLMProviderModel(ctx, id)
		mod.Enabled = false
		if err := st.UpdateLLMProviderModel(ctx, mod); err != nil {
			t.Fatal(err)
		}
	}
	_, err := st.PickLLMRotation(ctx, pairs)
	if err == nil || !strings.Contains(err.Error(), "no usable") {
		t.Fatalf("expected no usable: %v", err)
	}
}

func TestPickLLMRotation_picksFromUsable(t *testing.T) {
	st := openLLMStore(t)
	ctx := context.Background()
	p1, m1 := mustCreateLLMPair(t, st, ctx, "p1", "anthropic")
	p2, m2 := mustCreateLLMPair(t, st, ctx, "p2", "openai")
	pairs := []store.LLMPair{
		{ProviderID: p1, ModelID: m1},
		{ProviderID: p2, ModelID: m2},
	}

	got, err := st.PickLLMRotation(ctx, pairs)
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelID != m1 && got.ModelID != m2 {
		t.Fatalf("picked unknown: %+v", got)
	}
}

func TestPickLLMRotation_empty(t *testing.T) {
	st := openLLMStore(t)
	_, err := st.PickLLMRotation(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty: %v", err)
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
