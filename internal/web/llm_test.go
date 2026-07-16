package web

import (
	"context"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestParseLLMPairField(t *testing.T) {
	pid, mid, err := parseLLMPairField("0:0")
	if err != nil || pid != 0 || mid != 0 {
		t.Fatalf("empty: %d %d %v", pid, mid, err)
	}
	pid, mid, err = parseLLMPairField("3:9")
	if err != nil || pid != 3 || mid != 9 {
		t.Fatalf("pair: %d %d %v", pid, mid, err)
	}
	if _, _, err := parseLLMPairField("3"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := parseLLMPairField("1:0"); err == nil {
		t.Fatal("expected partial reject")
	}
}

func TestLLMPairOptionsIncludesDisabledCurrent(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	s := &Server{store: st}

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "off-prov", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
	}, "sk")
	if err != nil {
		t.Fatal(err)
	}
	mid, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: "m1", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.GetLLMProviderModel(ctx, mid)
	if err != nil {
		t.Fatal(err)
	}
	m.Enabled = false
	if err := st.UpdateLLMProviderModel(ctx, m); err != nil {
		t.Fatal(err)
	}

	opts, err := s.llmPairOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := formatLLMPair(pid, mid)
	for _, o := range opts {
		if o.Value == want {
			t.Fatal("disabled pair must not appear without current")
		}
	}

	opts, err = s.llmPairOptionsWithCurrent(ctx, pid, mid)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range opts {
		if o.Value == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected current disabled pair %q in %#v", want, opts)
	}
}
