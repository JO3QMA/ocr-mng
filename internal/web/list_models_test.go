package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestHTTPClientBlocksHTTPSDowngrade(t *testing.T) {
	first, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/models", nil)
	redirect, _ := http.NewRequest(http.MethodGet, "http://api.example.com/v1/models", nil)
	if err := httpClient.CheckRedirect(redirect, []*http.Request{first}); err == nil {
		t.Fatal("expected https→http downgrade to be blocked")
	}
}

func TestListModelsOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("auth %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "gpt-4"}, {"id": "gpt-3.5-turbo"}},
		})
	}))
	t.Cleanup(srv.Close)

	ids, err := ListModels(context.Background(), srv.URL+"/v1", ocr.ProtocolOpenAI, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "gpt-3.5-turbo" || ids[1] != "gpt-4" {
		t.Fatalf("got %#v", ids)
	}
}

func TestListModelsAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path %q", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "sk-ant" {
			t.Fatalf("key %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "claude-sonnet-4-6"}},
		})
	}))
	t.Cleanup(srv.Close)

	ids, err := ListModels(context.Background(), srv.URL, ocr.ProtocolAnthropic, "sk-ant")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "claude-sonnet-4-6" {
		t.Fatalf("got %#v", ids)
	}
}

func TestModelsURL(t *testing.T) {
	cases := []struct {
		in, protocol, want string
	}{
		{"https://api.openai.com/v1", ocr.ProtocolOpenAI, "https://api.openai.com/v1/models"},
		{"https://api.anthropic.com", ocr.ProtocolAnthropic, "https://api.anthropic.com/v1/models?limit=1000"},
		{"https://proxy.example/custom/v1/?tenant=abc", ocr.ProtocolAnthropic, "https://proxy.example/custom/v1/models?limit=1000&tenant=abc"},
		{"https://proxy.example/custom/v1/", ocr.ProtocolOpenAI, "https://proxy.example/custom/v1/models"},
		{"https://proxy.example/custom/v1/?tenant=abc", ocr.ProtocolOpenAI, "https://proxy.example/custom/v1/models?tenant=abc"},
		{"https://opencode.ai/zen/go/v1/chat/completions", ocr.ProtocolOpenAI, "https://opencode.ai/zen/go/v1/models"},
		{"https://opencode.ai/zen/go/v1/responses", ocr.ProtocolOpenAIResponses, "https://opencode.ai/zen/go/v1/models"},
		{"https://api.openai.com/v1/chat/completions", ocr.ProtocolOpenAI, "https://api.openai.com/v1/models"},
	}
	for _, tc := range cases {
		got, err := modelsURL(tc.in, tc.protocol, "")
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
	if _, err := modelsURL("", ocr.ProtocolOpenAI, ""); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestListModelsAnthropicPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			if r.URL.Query().Get("after_id") != "" {
				t.Fatalf("unexpected after_id on first page")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":     []map[string]string{{"id": "m1"}},
				"has_more": true,
				"last_id":  "m1",
			})
			return
		}
		if r.URL.Query().Get("after_id") != "m1" {
			t.Fatalf("after_id %q", r.URL.Query().Get("after_id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":     []map[string]string{{"id": "m2"}},
			"has_more": false,
		})
	}))
	t.Cleanup(srv.Close)

	ids, err := ListModels(context.Background(), srv.URL, ocr.ProtocolAnthropic, "sk-ant")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m2" {
		t.Fatalf("got %#v", ids)
	}
}

func TestListModelsStripsInferencePath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "gpt-5.6-luna"}},
		})
	}))
	t.Cleanup(srv.Close)

	base := strings.TrimPrefix(srv.URL, "http://")
	for _, apiBase := range []string{
		"http://" + base + "/zen/go/v1/chat/completions",
		"http://" + base + "/zen/go/v1/responses",
	} {
		gotPath = ""
		ids, err := ListModels(context.Background(), apiBase, ocr.ProtocolOpenAI, "sk-test")
		if err != nil {
			t.Fatalf("%s: %v", apiBase, err)
		}
		if gotPath != "/zen/go/v1/models" {
			t.Fatalf("%s: path %q want /zen/go/v1/models", apiBase, gotPath)
		}
		if len(ids) != 1 || ids[0] != "gpt-5.6-luna" {
			t.Fatalf("%s: got %#v", apiBase, ids)
		}
	}
}

func TestListModelsMissingDataField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	t.Cleanup(srv.Close)

	_, err := ListModels(context.Background(), srv.URL+"/v1", ocr.ProtocolOpenAI, "sk")
	if err == nil || !strings.Contains(err.Error(), "missing data field") {
		t.Fatalf("got %v", err)
	}
}
