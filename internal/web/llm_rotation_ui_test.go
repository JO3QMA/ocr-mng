package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/store"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

func TestBuildLLMRotationWidget(t *testing.T) {
	l := i18n.New("ja")
	opts := []llmPairOption{
		{Value: "1:2", Label: "A / m1"},
		{Value: "3:4", Label: "B / m2"},
	}
	w := buildLLMRotationWidget(l, "llm_pairs", false, opts, []store.LLMPair{{ProviderID: 1, ModelID: 2}})
	if w.FieldName != "llm_pairs" || w.RequireMin || len(w.Values) != 1 || w.Values[0] != "1:2" {
		t.Fatalf("widget: %#v", w)
	}
	if len(w.Config.Options) != 2 || w.Config.Labels.ModalTitle == "" {
		t.Fatalf("config: %#v", w.Config)
	}
	if w.ConfigJSON == "" || !strings.Contains(w.ConfigJSON, `"fieldName":"llm_pairs"`) {
		t.Fatalf("config json: %q", w.ConfigJSON)
	}
	b, err := json.Marshal(w.Config)
	if err != nil {
		t.Fatal(err)
	}
	var round llmRotationWidgetConfig
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	if round.FieldName != "llm_pairs" || len(round.Options) != 2 {
		t.Fatalf("round: %#v", round)
	}
}

func TestBuildLLMRotationWidget_requireMin(t *testing.T) {
	w := buildLLMRotationWidget(i18n.New("en"), "default_llm_pairs", true, nil, nil)
	if !w.RequireMin || !w.Config.RequireMin {
		t.Fatalf("require min: %#v", w)
	}
}

func TestParseSettingsForm_emptyLLMRotation(t *testing.T) {
	form := url.Values{
		"poll_interval_seconds":     {"60"},
		"min_poll_interval_seconds": {"30"},
		"max_concurrent_reviews":    {"2"},
		"review_run_retention_days": {"30"},
		"ui_language":               {"ja"},
		"review_language":           {"Japanese"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err := parseSettingsForm(req)
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty rotation error, got %v", err)
	}
}
