package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestPathID(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "/llm-providers/12/edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.SetPathValue("id", "12")
	id, ok := pathID(r, "id")
	if !ok || id != 12 {
		t.Fatalf("got %d %v", id, ok)
	}
	r.SetPathValue("id", "abc")
	if _, ok := pathID(r, "id"); ok {
		t.Fatal("expected reject")
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

func TestLLMProviderConnectionTest(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P", ProviderKey: "anthropic", Kind: "builtin", Enabled: false,
	}, "sk-stored-secret")
	if err != nil {
		t.Fatal(err)
	}
	mid, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: "m1", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho ok\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	s := &Server{store: st, ocrBinary: binary}

	form := url.Values{
		"name":          {"P"},
		"provider_key":  {"anthropic"},
		"kind":          {"builtin"},
		"test_model_id": {strconv.FormatInt(mid, 10)},
		"clear_api_key": {"on"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers/"+strconv.FormatInt(pid, 10)+"/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", strconv.FormatInt(pid, 10))
	rec := httptest.NewRecorder()
	s.llmProviderTest(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "接続に成功しました") && !strings.Contains(body, "Connection succeeded") {
		t.Fatalf("expected success flash, got: %s", body)
	}
	if strings.Contains(body, "sk-stored-secret") {
		t.Fatal("api key leaked into response")
	}
}

func TestLLMProviderConnectionTestNoModel(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &Server{store: st, ocrBinary: "ocr"}

	form := url.Values{
		"name":         {"P"},
		"provider_key": {"anthropic"},
		"kind":         {"builtin"},
		"api_key":      {"sk"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.llmProviderTest(rec, req)
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "モデルを指定") && !strings.Contains(string(body), "Specify a model") {
		t.Fatalf("expected no-model error: %s", body)
	}
}

func TestResolveConnectionTestModelInvalid(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P", ProviderKey: "anthropic", Kind: "builtin", Enabled: true,
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
	m, _ := st.GetLLMProviderModel(ctx, mid)
	m.Enabled = false
	_ = st.UpdateLLMProviderModel(ctx, m)

	s := &Server{store: st}
	_, err = s.resolveConnectionTestModel(ctx, pid, true, false, llmProviderFormView{
		SelectedModelID: mid,
	})
	if err == nil || err.Error() != "llm.test_model_invalid" {
		t.Fatalf("got %v", err)
	}
}
