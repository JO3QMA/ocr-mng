package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestParseLLMProviderFormBuiltinPreset(t *testing.T) {
	form := url.Values{
		"builtin_preset": {"anthropic"},
		"kind":           {"builtin"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p, _, err := parseLLMProviderForm(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.ProviderKey != "anthropic" || p.Name != "Anthropic" {
		t.Fatalf("got %#v", p)
	}

	form = url.Values{
		"builtin_preset": {ocr.BuiltinPresetOther},
		"provider_key":   {"my-gateway"},
		"name":           {"Gateway"},
		"kind":           {"builtin"},
	}
	req = httptest.NewRequest(http.MethodPost, "/llm-providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p, _, err = parseLLMProviderForm(req)
	if err != nil || p.ProviderKey != "my-gateway" || p.Name != "Gateway" {
		t.Fatalf("other: %#v %v", p, err)
	}

	form = url.Values{
		"builtin_preset": {"anthropic"},
		"provider_key":   {"ignored"},
		"name":           {"Keep"},
		"kind":           {"custom"},
	}
	req = httptest.NewRequest(http.MethodPost, "/llm-providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p, _, err = parseLLMProviderForm(req)
	if err != nil || p.ProviderKey != "ignored" || p.Name != "Keep" {
		t.Fatalf("custom ignores preset: %#v %v", p, err)
	}

	form = url.Values{
		"builtin_preset": {"tampered-key"},
		"provider_key":   {"fallback-key"},
		"name":           {"Fallback"},
		"kind":           {"builtin"},
	}
	req = httptest.NewRequest(http.MethodPost, "/llm-providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p, _, err = parseLLMProviderForm(req)
	if err != nil || p.ProviderKey != "fallback-key" || p.Name != "Fallback" {
		t.Fatalf("invalid preset uses provider_key: %#v %v", p, err)
	}

	form = url.Values{
		"builtin_preset": {ocr.BuiltinPresetOther},
		"provider_key":   {"my-key"},
		"kind":           {"builtin"},
	}
	req = httptest.NewRequest(http.MethodPost, "/llm-providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, _, err = parseLLMProviderForm(req)
	if err == nil || err.Error() != "llm.form_name_required" {
		t.Fatalf("expected name-only error, got %v", err)
	}

	form = url.Values{"kind": {"builtin"}}
	req = httptest.NewRequest(http.MethodPost, "/llm-providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, _, err = parseLLMProviderForm(req)
	if err == nil || err.Error() != "llm.form_name_and_provider_key_required" {
		t.Fatalf("expected both missing error, got %v", err)
	}
}

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

	opts, err = s.llmPairOptionsWithCurrents(ctx, []store.LLMPair{{ProviderID: pid, ModelID: mid}})
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

func TestUndiscoveredModelNames(t *testing.T) {
	ledger := []store.LLMProviderModel{
		{ModelName: "gpt-4"},
		{ModelName: "old", Enabled: false},
	}
	got := undiscoveredModelNames(ledger, []string{"gpt-4", "gpt-3.5-turbo", "old", "GPT-4"})
	if len(got) != 1 || got[0] != "gpt-3.5-turbo" {
		t.Fatalf("got %#v", got)
	}
}

func TestLLMProviderModelsDiscover(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "gpt-4"},
				{"id": "gpt-3.5-turbo"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "P", ProviderKey: "openai", Kind: "custom",
		APIBaseURL: srv.URL + "/v1", Protocol: "openai", Enabled: true,
	}, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateLLMProviderModel(ctx, store.LLMProviderModel{
		ProviderID: pid, ModelName: "gpt-4", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	s := &Server{store: st}
	form := url.Values{
		"name":          {"P"},
		"provider_key":  {"openai"},
		"kind":          {"custom"},
		"api_base_url":  {srv.URL + "/v1"},
		"protocol":      {"openai"},
		"api_key":       {"sk-test"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers/"+strconv.FormatInt(pid, 10)+"/models/discover", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", strconv.FormatInt(pid, 10))
	rec := httptest.NewRecorder()
	s.llmProviderModelsDiscover(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<option value="gpt-3.5-turbo">`) {
		t.Fatalf("expected remote model option: %s", body)
	}
	if strings.Contains(body, `<option value="gpt-4">`) {
		t.Fatalf("registered model should be excluded from pick list: %s", body)
	}
	if strings.Contains(body, "sk-test") {
		t.Fatal("api key leaked")
	}
}

func TestLLMProviderModelsDiscoverNoURL(t *testing.T) {
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

	s := &Server{store: st}
	form := url.Values{
		"name":         {"P"},
		"provider_key": {"anthropic"},
		"kind":         {"builtin"},
		"api_key":      {"sk"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers/"+strconv.FormatInt(pid, 10)+"/models/discover", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", strconv.FormatInt(pid, 10))
	rec := httptest.NewRecorder()
	s.llmProviderModelsDiscover(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "API Base URL") && !strings.Contains(body, "enter a model name manually") {
		t.Fatalf("expected no-url message: %s", body)
	}
}

func TestParseLLMProviderConnectionFormSkipsNameValidation(t *testing.T) {
	form := url.Values{
		"kind":         {"custom"},
		"api_base_url": {"https://example/v1"},
		"protocol":     {"openai"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers/1/models/discover", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p, _, err := parseLLMProviderConnectionForm(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.APIBaseURL != "https://example/v1" || p.Name != "" || p.ProviderKey != "" {
		t.Fatalf("got %#v", p)
	}
}

func TestLLMProviderModelsDiscoverAllowsEmptyName(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "gpt-3.5-turbo"}},
		})
	}))
	t.Cleanup(srv.Close)

	pid, err := st.CreateLLMProvider(ctx, store.LLMProvider{
		Name: "Stored", ProviderKey: "openai", Kind: "custom",
		APIBaseURL: srv.URL + "/v1", Protocol: "openai", Enabled: true,
	}, "sk-test")
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{store: st}
	form := url.Values{
		"name":         {""},
		"provider_key": {""},
		"kind":         {"custom"},
		"api_base_url": {srv.URL + "/v1"},
		"protocol":     {"openai"},
		"api_key":      {"sk-test"},
	}
	req := httptest.NewRequest(http.MethodPost, "/llm-providers/"+strconv.FormatInt(pid, 10)+"/models/discover", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", strconv.FormatInt(pid, 10))
	rec := httptest.NewRecorder()
	s.llmProviderModelsDiscover(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "名前") && strings.Contains(body, "flash error") {
		t.Fatalf("name validation should not block discover: %s", body)
	}
	if !strings.Contains(body, `<option value="gpt-3.5-turbo">`) {
		t.Fatalf("expected discover success: %s", body)
	}
}
