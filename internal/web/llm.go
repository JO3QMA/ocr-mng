package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/store"
)

const llmConnectionTestTimeout = 30 * time.Second

type llmPairOption struct {
	ProviderID int64
	ModelID    int64
	Value      string
	Label      string
}

type llmProviderFormView struct {
	page
	Provider               store.LLMProvider
	Models                 []store.LLMProviderModel
	EnabledModels          []store.LLMProviderModel
	UseTempModel           bool
	SelectedModelID        int64
	TempModelName          string
	FormTitle              string
	FormTitleKey           string
	KeyHintKey             string
	Action                 string
	TestAction             string
	ErrMsg                 string
	TestOK                 bool
	TestMsg                string
	DiscoverAction         string
	DiscoverOK             bool
	DiscoverMsg            string
	RemoteModels           []string
	KeyHint                string
	ShowClearKey           bool
	BuiltinPreset          string
	BuiltinPresets         []ocr.BuiltinPreset
	BuiltinProviderDocsURL string
}

func (s *Server) llmPairOptions(ctx context.Context) ([]llmPairOption, error) {
	return s.llmPairOptionsWithCurrents(ctx, nil)
}

// llmPairOptionsWithCurrents lists enabled pairs, and always includes current
// pairs (even if disabled) so settings/repo forms cannot drop them.
func (s *Server) llmPairOptionsWithCurrents(ctx context.Context, currents []store.LLMPair) ([]llmPairOption, error) {
	providers, err := s.store.ListLLMProviders(ctx)
	if err != nil {
		return nil, err
	}
	meta := map[string]string{}
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
			val := formatLLMPair(p.ID, m.ID)
			label := p.Name + " / " + m.ModelName
			if !p.Enabled || !m.Enabled {
				meta[val] = label + " (disabled)"
				continue
			}
			meta[val] = label
			add(llmPairOption{
				ProviderID: p.ID,
				ModelID:    m.ID,
				Value:      val,
				Label:      label,
			})
		}
	}
	for _, cur := range currents {
		if cur.ProviderID == 0 || cur.ModelID == 0 {
			continue
		}
		val := formatLLMPair(cur.ProviderID, cur.ModelID)
		if seen[val] {
			continue
		}
		label := val
		if m, ok := meta[val]; ok {
			label = m
		} else if l, err := s.llmPairLabel(ctx, cur.ProviderID, cur.ModelID); err == nil {
			label = l
		}
		add(llmPairOption{
			ProviderID: cur.ProviderID,
			ModelID:    cur.ModelID,
			Value:      val,
			Label:      label,
		})
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

func parseLLMPairsFields(values []string) ([]store.LLMPair, error) {
	var out []store.LLMPair
	seen := map[string]struct{}{}
	for _, v := range values {
		pid, mid, err := parseLLMPairField(v)
		if err != nil {
			return nil, err
		}
		if pid == 0 && mid == 0 {
			continue
		}
		key := formatLLMPair(pid, mid)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate llm pair in rotation set")
		}
		seen[key] = struct{}{}
		out = append(out, store.LLMPair{ProviderID: pid, ModelID: mid})
	}
	return out, nil
}

func formatLLMPair(providerID, modelID int64) string {
	if providerID == 0 && modelID == 0 {
		return "0:0"
	}
	return fmt.Sprintf("%d:%d", providerID, modelID)
}

func llmRotationValues(pairs []store.LLMPair) []string {
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = formatLLMPair(p.ProviderID, p.ModelID)
	}
	return out
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
	s.renderLLMProviderForm(w, r, llmProviderFormView{
		Provider: store.LLMProvider{Kind: "builtin", Enabled: true},
		Action:   "/llm-providers", TestAction: "/llm-providers/test",
		FormTitleKey: "page.new_llm_provider", KeyHintKey: "form.pat_optional",
		UseTempModel: true,
	})
}

func (s *Server) llmProviderCreate(w http.ResponseWriter, r *http.Request) {
	p, apiKey, err := parseLLMProviderForm(r)
	p.Enabled = true // create form has no enabled toggle
	view := llmProviderFormView{
		Provider: p, Action: "/llm-providers", TestAction: "/llm-providers/test",
		FormTitleKey: "page.new_llm_provider", KeyHintKey: "form.pat_optional",
		UseTempModel: true, TempModelName: strings.TrimSpace(r.FormValue("temp_model_name")),
	}
	if err != nil {
		view.ErrMsg = s.llmFormErrMsg(r, err)
		s.renderLLMProviderForm(w, r, view)
		return
	}
	id, err := s.store.CreateLLMProvider(r.Context(), p, apiKey)
	if err != nil {
		view.ErrMsg = err.Error()
		s.renderLLMProviderForm(w, r, view)
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
	enabled := enabledLLMModels(models)
	view := s.llmProviderEditFormView(r, id, p.HasAPIKey, p, models, enabled)
	s.renderLLMProviderForm(w, r, view)
}

func (s *Server) llmProviderUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, apiKey, err := parseLLMProviderForm(r)
	p.ID = id
	stored, getErr := s.store.GetLLMProvider(r.Context(), id)
	if getErr != nil {
		http.NotFound(w, r)
		return
	}
	p.HasAPIKey = stored.HasAPIKey
	models, listErr := s.store.ListLLMProviderModels(r.Context(), id)
	if listErr != nil {
		view := s.llmProviderEditFormView(r, id, stored.HasAPIKey, p, nil, nil)
		view.ErrMsg = listErr.Error()
		s.renderLLMProviderForm(w, r, view)
		return
	}
	enabled := enabledLLMModels(models)
	failView := s.llmProviderEditFormView(r, id, stored.HasAPIKey, p, models, enabled)
	if err != nil {
		failView.ErrMsg = s.llmFormErrMsg(r, err)
		s.renderLLMProviderForm(w, r, failView)
		return
	}
	if err := s.store.UpdateLLMProvider(r.Context(), p, apiKey, r.FormValue("clear_api_key") == "on"); err != nil {
		failView.ErrMsg = err.Error()
		s.renderLLMProviderForm(w, r, failView)
		return
	}
	http.Redirect(w, r, "/llm-providers?flash=updated", http.StatusSeeOther)
}

func (s *Server) llmProviderTest(w http.ResponseWriter, r *http.Request) {
	p, formKey, err := parseLLMProviderForm(r)
	providerID, hasID := pathID(r, "id")
	var models []store.LLMProviderModel
	var hasStoredKey bool
	if hasID {
		p.ID = providerID
		stored, getErr := s.store.GetLLMProvider(r.Context(), providerID)
		if getErr != nil {
			http.NotFound(w, r)
			return
		}
		hasStoredKey = stored.HasAPIKey
		var listErr error
		models, listErr = s.store.ListLLMProviderModels(r.Context(), providerID)
		if listErr != nil {
			p.HasAPIKey = hasStoredKey
			view := llmProviderFormView{
				Provider: p, UseTempModel: true,
				TempModelName:   strings.TrimSpace(r.FormValue("temp_model_name")),
				SelectedModelID: formModelID(r),
				FormTitleKey:    "page.edit_llm_provider",
				Action:          fmt.Sprintf("/llm-providers/%d", providerID),
				TestAction:      fmt.Sprintf("/llm-providers/%d/test", providerID),
				ShowClearKey:    hasStoredKey,
				TestMsg:         listErr.Error(),
			}
			if hasStoredKey {
				view.KeyHintKey = "form.pat_keep"
			} else {
				view.KeyHintKey = "form.pat_required"
			}
			s.renderLLMProviderForm(w, r, view)
			return
		}
		p.HasAPIKey = hasStoredKey
	}
	enabled := enabledLLMModels(models)
	useTemp := !hasID || len(enabled) == 0
	loc := s.page(r, "page.new_llm_provider").L
	view := llmProviderFormView{
		Provider: p, Models: models, EnabledModels: enabled,
		UseTempModel: useTemp, TempModelName: strings.TrimSpace(r.FormValue("temp_model_name")),
		SelectedModelID: formModelID(r),
		FormTitleKey:    "page.new_llm_provider", KeyHintKey: "form.pat_optional",
		Action: "/llm-providers", TestAction: "/llm-providers/test",
	}
	if hasID {
		view.FormTitleKey = "page.edit_llm_provider"
		view.Action = fmt.Sprintf("/llm-providers/%d", providerID)
		view.TestAction = fmt.Sprintf("/llm-providers/%d/test", providerID)
		view.ShowClearKey = hasStoredKey
		if hasStoredKey {
			view.KeyHintKey = "form.pat_keep"
		} else {
			view.KeyHintKey = "form.pat_required"
		}
		if len(enabled) == 1 && view.SelectedModelID == 0 {
			view.SelectedModelID = enabled[0].ID
		}
		loc = s.page(r, view.FormTitleKey).L
	}
	if err != nil {
		view.TestMsg = s.llmFormErrMsg(r, err)
		s.renderLLMProviderForm(w, r, view)
		return
	}

	apiKey := formKey
	if apiKey == "" && hasID {
		apiKey, err = s.store.LLMProviderAPIKey(r.Context(), providerID)
		if err != nil {
			view.TestMsg = err.Error()
			s.renderLLMProviderForm(w, r, view)
			return
		}
	}
	if apiKey == "" {
		view.TestMsg = loc.T("llm.test_no_key")
		s.renderLLMProviderForm(w, r, view)
		return
	}

	modelName, modelErr := s.resolveConnectionTestModel(r.Context(), providerID, hasID, useTemp, view)
	if modelErr != nil {
		key := modelErr.Error()
		if key == "llm.test_no_model" || key == "llm.test_model_invalid" {
			view.TestMsg = loc.T(key)
		} else {
			view.TestMsg = key
		}
		s.renderLLMProviderForm(w, r, view)
		return
	}

	configJSON, err := ocr.BuildProviderConfig(p.Kind, p.ProviderKey, apiKey, p.APIBaseURL, p.Protocol, modelName, "")
	if err != nil {
		view.TestMsg = err.Error()
		s.renderLLMProviderForm(w, r, view)
		return
	}

	homeDir, err := os.MkdirTemp("", "ocr-mng-llm-test-*")
	if err != nil {
		view.TestMsg = err.Error()
		s.renderLLMProviderForm(w, r, view)
		return
	}
	defer func() { _ = os.RemoveAll(homeDir) }()

	ctx, cancel := context.WithTimeout(r.Context(), llmConnectionTestTimeout)
	defer cancel()
	runner := ocr.Runner{Binary: s.ocrBinary, HomeDir: homeDir, ConfigJSON: configJSON}
	if err := runner.TestLLM(ctx); err != nil {
		msg := ocr.MaskSecret(err.Error(), apiKey)
		if ctx.Err() != nil {
			msg = loc.T("llm.test_timeout")
		}
		view.TestMsg = msg
		s.renderLLMProviderForm(w, r, view)
		return
	}
	view.TestOK = true
	view.TestMsg = fmt.Sprintf(loc.T("llm.test_ok"), modelName)
	s.renderLLMProviderForm(w, r, view)
}

func (s *Server) llmProviderModelsDiscover(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	stored, err := s.store.GetLLMProvider(r.Context(), providerID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	models, listErr := s.store.ListLLMProviderModels(r.Context(), providerID)
	enabled := enabledLLMModels(models)
	p, formKey, err := parseLLMProviderConnectionForm(r)
	p = mergeDiscoverProvider(stored, p)
	p.ID = providerID
	p.HasAPIKey = stored.HasAPIKey
	if err != nil {
		view := s.llmProviderEditFormView(r, providerID, stored.HasAPIKey, p, models, enabled)
		view.ErrMsg = s.llmFormErrMsg(r, err)
		s.renderLLMProviderForm(w, r, view)
		return
	}
	if listErr != nil {
		view := s.llmProviderEditFormView(r, providerID, stored.HasAPIKey, p, nil, nil)
		view.ErrMsg = listErr.Error()
		s.renderLLMProviderForm(w, r, view)
		return
	}
	view := s.llmProviderEditFormView(r, providerID, stored.HasAPIKey, p, models, enabled)
	loc := s.page(r, view.FormTitleKey).L
	if strings.TrimSpace(p.APIBaseURL) == "" {
		view.DiscoverMsg = loc.T("llm.discover_no_url")
		s.renderLLMProviderForm(w, r, view)
		return
	}
	apiKey := formKey
	if apiKey == "" {
		apiKey, err = s.store.LLMProviderAPIKey(r.Context(), providerID)
		if err != nil {
			view.DiscoverMsg = err.Error()
			s.renderLLMProviderForm(w, r, view)
			return
		}
	}
	if apiKey == "" {
		view.DiscoverMsg = loc.T("llm.test_no_key")
		s.renderLLMProviderForm(w, r, view)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmConnectionTestTimeout)
	defer cancel()
	remote, err := ListModels(ctx, p.APIBaseURL, p.Protocol, apiKey)
	if err != nil {
		msg := ocr.MaskSecret(err.Error(), apiKey)
		if ctx.Err() != nil {
			msg = loc.T("llm.discover_timeout")
		}
		view.DiscoverMsg = msg
		s.renderLLMProviderForm(w, r, view)
		return
	}
	view.RemoteModels = undiscoveredModelNames(models, remote)
	if len(view.RemoteModels) == 0 {
		view.DiscoverMsg = loc.T("llm.discover_none")
	} else {
		view.DiscoverOK = true
		view.DiscoverMsg = fmt.Sprintf(loc.T("llm.discover_ok"), len(view.RemoteModels))
	}
	s.renderLLMProviderForm(w, r, view)
}

func (s *Server) llmProviderEditFormView(r *http.Request, providerID int64, hasStoredKey bool, p store.LLMProvider, models []store.LLMProviderModel, enabled []store.LLMProviderModel) llmProviderFormView {
	keyHint := "form.pat_required"
	if hasStoredKey {
		keyHint = "form.pat_keep"
	}
	view := llmProviderFormView{
		Provider: p, Models: models, EnabledModels: enabled,
		Action:         fmt.Sprintf("/llm-providers/%d", providerID),
		TestAction:     fmt.Sprintf("/llm-providers/%d/test", providerID),
		DiscoverAction: fmt.Sprintf("/llm-providers/%d/models/discover", providerID),
		FormTitleKey:   "page.edit_llm_provider", KeyHintKey: keyHint, ShowClearKey: hasStoredKey,
		UseTempModel:    len(enabled) == 0,
		TempModelName:   strings.TrimSpace(r.FormValue("temp_model_name")),
		SelectedModelID: formModelID(r),
	}
	if len(enabled) == 1 && view.SelectedModelID == 0 {
		view.SelectedModelID = enabled[0].ID
	}
	return view
}

func undiscoveredModelNames(ledger []store.LLMProviderModel, remote []string) []string {
	normalized := map[string]struct{}{}
	for _, m := range ledger {
		normalized[strings.ToLower(strings.TrimSpace(m.ModelName))] = struct{}{}
	}
	var out []string
	for _, name := range remote {
		if _, ok := normalized[strings.ToLower(strings.TrimSpace(name))]; ok {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (s *Server) resolveConnectionTestModel(ctx context.Context, providerID int64, hasID, useTemp bool, view llmProviderFormView) (string, error) {
	if useTemp {
		name := strings.TrimSpace(view.TempModelName)
		if name == "" {
			return "", fmt.Errorf("llm.test_no_model")
		}
		return name, nil
	}
	mid := view.SelectedModelID
	if mid == 0 && len(view.EnabledModels) == 1 {
		mid = view.EnabledModels[0].ID
	}
	if mid == 0 {
		return "", fmt.Errorf("llm.test_no_model")
	}
	m, err := s.store.GetLLMProviderModel(ctx, mid)
	if err != nil || !hasID || m.ProviderID != providerID || !m.Enabled {
		return "", fmt.Errorf("llm.test_model_invalid")
	}
	return m.ModelName, nil
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

func (s *Server) llmModelsBulkUpdate(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	redirectInvalid := func() {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=invalid_model", pid), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		redirectInvalid()
		return
	}
	var pending []store.LLMProviderModel
	for _, idStr := range r.Form["model_id"] {
		mid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			redirectInvalid()
			return
		}
		m, err := s.store.GetLLMProviderModel(r.Context(), mid)
		if err != nil || m.ProviderID != pid {
			redirectInvalid()
			return
		}
		if name := strings.TrimSpace(r.FormValue("model_name_" + idStr)); name != "" {
			m.ModelName = name
		}
		m.Enabled = r.FormValue("enabled_"+idStr) == "on"
		pending = append(pending, m)
	}
	if len(pending) == 0 {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit", pid), http.StatusSeeOther)
		return
	}
	for _, m := range pending {
		if err := s.store.UpdateLLMProviderModel(r.Context(), m); err != nil {
			redirectInvalid()
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=updated", pid), http.StatusSeeOther)
}

func (s *Server) llmModelsBulkDelete(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	redirectFailed := func() {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=delete_failed", pid), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		redirectFailed()
		return
	}
	var pending []int64
	for _, idStr := range r.Form["delete_model_id"] {
		mid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			redirectFailed()
			return
		}
		m, err := s.store.GetLLMProviderModel(r.Context(), mid)
		if err != nil || m.ProviderID != pid {
			redirectFailed()
			return
		}
		pending = append(pending, mid)
	}
	if len(pending) == 0 {
		http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit", pid), http.StatusSeeOther)
		return
	}
	for _, mid := range pending {
		if err := s.store.DeleteLLMProviderModel(r.Context(), mid); err != nil {
			redirectFailed()
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/llm-providers/%d/edit?flash=deleted", pid), http.StatusSeeOther)
}

func (s *Server) renderLLMProviderForm(w http.ResponseWriter, r *http.Request, v llmProviderFormView) {
	pge := s.page(r, v.FormTitleKey)
	v.page = pge
	v.FormTitle = pge.Title
	if v.KeyHintKey != "" {
		v.KeyHint = pge.L.T(v.KeyHintKey)
	}
	if v.EnabledModels == nil {
		v.EnabledModels = enabledLLMModels(v.Models)
	}
	if v.BuiltinPreset == "" {
		v.BuiltinPreset = formBuiltinPreset(r, v.Provider.ProviderKey)
	}
	if v.Provider.ID != 0 && v.DiscoverAction == "" {
		v.DiscoverAction = fmt.Sprintf("/llm-providers/%d/models/discover", v.Provider.ID)
	}
	v.BuiltinPresets = ocr.BuiltinPresets()
	v.BuiltinProviderDocsURL = ocr.BuiltinProviderDocsURL(pge.Lang)
	render(w, "llm_provider_form", v)
}

func enabledLLMModels(models []store.LLMProviderModel) []store.LLMProviderModel {
	var out []store.LLMProviderModel
	for _, m := range models {
		if m.Enabled {
			out = append(out, m)
		}
	}
	return out
}

func formModelID(r *http.Request) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("test_model_id")), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func formBuiltinPreset(r *http.Request, providerKey string) string {
	if r != nil {
		if v := strings.TrimSpace(r.FormValue("builtin_preset")); v != "" {
			return v
		}
	}
	return ocr.SelectedBuiltinPreset(providerKey)
}

func (s *Server) llmFormErrMsg(r *http.Request, err error) string {
	if err == nil {
		return ""
	}
	return s.page(r, "page.llm_providers").L.T(err.Error())
}

func parseLLMProviderConnectionForm(r *http.Request) (store.LLMProvider, string, error) {
	p, apiKey, err := parseLLMProviderFields(r)
	if err != nil {
		return p, apiKey, err
	}
	if p.Kind != "builtin" && p.Kind != "custom" {
		return p, "", fmt.Errorf("llm.form_kind_invalid")
	}
	return p, apiKey, nil
}

func mergeDiscoverProvider(stored, form store.LLMProvider) store.LLMProvider {
	if strings.TrimSpace(form.Name) == "" {
		form.Name = stored.Name
	}
	if strings.TrimSpace(form.ProviderKey) == "" {
		form.ProviderKey = stored.ProviderKey
	}
	if strings.TrimSpace(form.Kind) == "" {
		form.Kind = stored.Kind
	}
	if strings.TrimSpace(form.APIBaseURL) == "" {
		form.APIBaseURL = stored.APIBaseURL
	}
	if strings.TrimSpace(form.Protocol) == "" {
		form.Protocol = stored.Protocol
	}
	return form
}

func parseLLMProviderForm(r *http.Request) (store.LLMProvider, string, error) {
	p, apiKey, err := parseLLMProviderFields(r)
	if err != nil {
		return p, apiKey, err
	}
	var missingKey string
	switch {
	case p.Name == "" && p.ProviderKey == "":
		missingKey = "llm.form_name_and_provider_key_required"
	case p.Name == "":
		missingKey = "llm.form_name_required"
	case p.ProviderKey == "":
		missingKey = "llm.form_provider_key_required"
	}
	if missingKey != "" {
		return p, "", fmt.Errorf("%s", missingKey)
	}
	if p.Kind != "builtin" && p.Kind != "custom" {
		return p, "", fmt.Errorf("llm.form_kind_invalid")
	}
	return p, apiKey, nil
}

func parseLLMProviderFields(r *http.Request) (store.LLMProvider, string, error) {
	if err := r.ParseForm(); err != nil {
		return store.LLMProvider{}, "", err
	}
	preset := strings.TrimSpace(r.FormValue("builtin_preset"))
	p := store.LLMProvider{
		Name:       strings.TrimSpace(r.FormValue("name")),
		Kind:       strings.TrimSpace(r.FormValue("kind")),
		APIBaseURL: strings.TrimSpace(r.FormValue("api_base_url")),
		Protocol:   strings.TrimSpace(r.FormValue("protocol")),
		Enabled:    r.FormValue("enabled") == "on",
	}
	if p.Kind == "" {
		p.Kind = "builtin"
	}
	_, presetKnown := ocr.BuiltinPresetLabel(preset)
	usePreset := p.Kind == "builtin" && preset != "" && preset != ocr.BuiltinPresetOther && presetKnown
	if usePreset {
		p.ProviderKey = preset
	} else {
		p.ProviderKey = strings.TrimSpace(r.FormValue("provider_key"))
	}
	if p.Name == "" && usePreset {
		if label, ok := ocr.BuiltinPresetLabel(preset); ok {
			p.Name = label
		}
	}
	return p, strings.TrimSpace(r.FormValue("api_key")), nil
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil
}
