package llmclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

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
		{"https://proxy.example/custom/v1/", ocr.ProtocolOpenAI, "https://proxy.example/custom/v1/models"},
	}
	for _, tc := range cases {
		got, err := modelsURL(tc.in, tc.protocol)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
	if _, err := modelsURL("", ocr.ProtocolOpenAI); err == nil {
		t.Fatal("expected error for empty url")
	}
}
