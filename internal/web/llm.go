package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
)

type llmPairOption struct {
	ProviderID int64
	ModelID    int64
	Value      string
	Label      string
}

func (s *Server) llmPairOptions(ctx context.Context) ([]llmPairOption, error) {
	return s.llmPairOptionsWithCurrent(ctx, 0, 0)
}

// llmPairOptionsWithCurrent lists enabled pairs, and always includes the current
// provider/model pair (even if disabled) so settings/repo forms cannot drop it.
func (s *Server) llmPairOptionsWithCurrent(ctx context.Context, curProviderID, curModelID int64) ([]llmPairOption, error) {
	providers, err := s.store.ListLLMProviders(ctx)
	if err != nil {
		return nil, err
	}
	var out []llmPairOption
	seen := map[string]bool{}
	add := func(opt llmPairOption) {
		if seen[opt.Value] {
			return
		}
		seen[opt.Value] = true
		out = append(out, opt)
	}
	for _, p := range providers {
		models, err := s.store.ListLLMProviderModels(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range models {
			if !p.Enabled || !m.Enabled {
				continue
			}
			add(llmPairOption{
				ProviderID: p.ID,
				ModelID:    m.ID,
				Value:      formatLLMPair(p.ID, m.ID),
				Label:      p.Name + " / " + m.ModelName,
			})
		}
	}
	if curProviderID != 0 && curModelID != 0 {
		val := formatLLMPair(curProviderID, curModelID)
		if !seen[val] {
			label, err := s.llmPairLabel(ctx, curProviderID, curModelID)
			if err != nil {
				label = val
			}
			add(llmPairOption{
				ProviderID: curProviderID,
				ModelID:    curModelID,
				Value:      val,
				Label:      label,
			})
		}
	}
	return out, nil
}

func (s *Server) llmPairLabel(ctx context.Context, providerID, modelID int64) (string, error) {
	p, err := s.store.GetLLMProvider(ctx, providerID)
	if err != nil {
		return "", err
	}
	m, err := s.store.GetLLMProviderModel(ctx, modelID)
	if err != nil {
		return "", err
	}
	suffix := ""
	if !p.Enabled || !m.Enabled {
		suffix = " (disabled)"
	}
	return p.Name + " / " + m.ModelName + suffix, nil
}

func parseLLMPairField(v string) (providerID, modelID int64, err error) {
	v = strings.TrimSpace(v)
	if v == "" || v == "0" || v == "0:0" {
		return 0, 0, nil
	}
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid llm pair")
	}
	providerID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid llm pair")
	}
	modelID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid llm pair")
	}
	return providerID, modelID, store.ValidateLLMPairIDs(providerID, modelID)
}

func formatLLMPair(providerID, modelID int64) string {
	if providerID == 0 && modelID == 0 {
		return "0:0"
	}
	return fmt.Sprintf("%d:%d", providerID, modelID)
}

func (s *Server) llmProvidersList(w http.ResponseWriter, r *http.Request) {
	providers, err := s.store.ListLLMProviders(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	render(w, "llm_providers", struct {
		page
		Providers []store.LLMProvider
	}{page: s.page(r, "page.llm_providers"), Providers: providers})
}

func (s *Server) llmProviderNew(w http.ResponseWriter, r *http.Request) {
	s.renderLLMProviderForm(w, r, store.LLMProvider{Kind: "builtin", Enabled: true}, nil, "", "/llm-providers", "page.new_llm_provider", "form.pat_optional", false)
}

func (s *Server) llmProviderCreate(w http.ResponseWriter, r *http.Request) {
	p, apiKey, err := parseLLMProviderForm(r)
	if err != nil {
		s.renderLLMProviderForm(w, r, p, nil, err.Error(), "/llm-providers", "page.new_llm_provider", "form.pat_optional", false)
		return
	}
	p.Enabled = true // create form has no enabled toggle
	id, err := s.store.CreateLLMProvider(r.Context(), p, apiKey)
	if err != nil {
		s.renderLLMProviderForm(w, r, p, nil, err.Error(), "/llm-providers", "page.new_llm_provider", "form.pat_optional", false)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=created", id), http.StatusSeeOther)
}

func (s *Server) llmProviderEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := s.store.GetLLMProvider(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	models, err := s.store.ListLLMProviderModels(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	keyHint := "form.pat_required"
	if p.HasAPIKey {
		keyHint = "form.pat_keep"
	}
	s.renderLLMProviderForm(w, r, p, models, "", fmt.Sprintf("/llm-providers/%d", id), "page.edit_llm_provider", keyHint, p.HasAPIKey)
}

func (s *Server) llmProviderUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, apiKey, err := parseLLMProviderForm(r)
	if err != nil {
		models, _ := s.store.ListLLMProviderModels(r.Context(), id)
		s.renderLLMProviderForm(w, r, p, models, err.Error(), fmt.Sprintf("/llm-providers/%d", id), "page.edit_llm_provider", "", false)
		return
	}
	p.ID = id
	if err := s.store.UpdateLLMProvider(r.Context(), p, apiKey, r.FormValue("clear_api_key") == "on"); err != nil {
		models, _ := s.store.ListLLMProviderModels(r.Context(), id)
		s.renderLLMProviderForm(w, r, p, models, err.Error(), fmt.Sprintf("/llm-providers/%d", id), "page.edit_llm_provider", "", false)
		return
	}
	http.Redirect(w, r, "/llm-providers?flash=updated", http.StatusSeeOther)
}

func (s *Server) llmProviderDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.store.DeleteLLMProvider(r.Context(), id); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=delete_failed", id), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/llm-providers?flash=deleted", http.StatusSeeOther)
}

func (s *Server) llmModelCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimSpace(r.FormValue("model_name"))
	if name == "" {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=invalid_model", id), http.StatusSeeOther)
		return
	}
	_, err := s.store.CreateLLMProviderModel(r.Context(), store.LLMProviderModel{
		ProviderID: id, ModelName: name, Enabled: true,
	})
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=invalid_model", id), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=created", id), http.StatusSeeOther)
}

func (s *Server) llmModelUpdate(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	mid, ok2 := pathID(r, "mid")
	if !ok || !ok2 {
		http.NotFound(w, r)
		return
	}
	m, err := s.store.GetLLMProviderModel(r.Context(), mid)
	if err != nil || m.ProviderID != pid {
		http.NotFound(w, r)
		return
	}
	if name := strings.TrimSpace(r.FormValue("model_name")); name != "" {
		m.ModelName = name
	}
	m.Enabled = r.FormValue("enabled") == "on"
	if err := s.store.UpdateLLMProviderModel(r.Context(), m); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=invalid_model", pid), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=updated", pid), http.StatusSeeOther)
}

func (s *Server) llmModelDelete(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	mid, ok2 := pathID(r, "mid")
	if !ok || !ok2 {
		http.NotFound(w, r)
		return
	}
	m, err := s.store.GetLLMProviderModel(r.Context(), mid)
	if err != nil || m.ProviderID != pid {
		http.NotFound(w, r)
		return
	}
	if err := s.store.DeleteLLMProviderModel(r.Context(), mid); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=delete_failed", pid), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=deleted", pid), http.StatusSeeOther)
}

func (s *Server) renderLLMProviderForm(w http.ResponseWriter, r *http.Request, p store.LLMProvider, models []store.LLMProviderModel, errMsg, action, titleKey, keyHintKey string, showClear bool) {
	pge := s.page(r, titleKey)
	render(w, "llm_provider_form", struct {
		page
		Provider     store.LLMProvider
		Models       []store.LLMProviderModel
		FormTitle    string
		Action       string
		ErrMsg       string
		KeyHint      string
		ShowClearKey bool
	}{
		page: pge, Provider: p, Models: models, FormTitle: pge.Title, Action: action,
		ErrMsg: errMsg, KeyHint: pge.L.T(keyHintKey), ShowClearKey: showClear,
	})
}

func parseLLMProviderForm(r *http.Request) (store.LLMProvider, string, error) {
	if err := r.ParseForm(); err != nil {
		return store.LLMProvider{}, "", err
	}
	p := store.LLMProvider{
		Name:        strings.TrimSpace(r.FormValue("name")),
		ProviderKey: strings.TrimSpace(r.FormValue("provider_key")),
		Kind:        strings.TrimSpace(r.FormValue("kind")),
		APIBaseURL:  strings.TrimSpace(r.FormValue("api_base_url")),
		Protocol:    strings.TrimSpace(r.FormValue("protocol")),
		Enabled:     r.FormValue("enabled") == "on",
	}
	if p.Name == "" || p.ProviderKey == "" {
		return p, "", fmt.Errorf("name and provider_key are required")
	}
	if p.Kind == "" {
		p.Kind = "builtin"
	}
	if p.Kind != "builtin" && p.Kind != "custom" {
		return p, "", fmt.Errorf("kind must be builtin or custom")
	}
	return p, strings.TrimSpace(r.FormValue("api_key")), nil
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil
}

// ledgerModeActive mirrors review.LedgerMode for templates/handlers.
func ledgerModeActive(gs store.GlobalSettings) bool {
	return review.LedgerMode(gs)
}
