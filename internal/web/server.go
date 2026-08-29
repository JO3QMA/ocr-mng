package web

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
	"github.com/jo3qma/ocr-mng/internal/web/i18n"
)

type Server struct {
	adminUser string
	adminPass string
	store     *store.Store
	engine    *review.Engine
	ocrBinary string
}

type page struct {
	Title string
	Flash string
	Lang  string
	L     i18n.Localizer
}

func (s *Server) page(r *http.Request, titleKey string) page {
	gs, _ := s.store.GetGlobalSettings(r.Context())
	loc := i18n.New(gs.UILanguage)
	p := page{Lang: gs.UILanguage, L: loc, Title: loc.T(titleKey)}
	if fk := r.URL.Query().Get("flash"); fk != "" {
		p.Flash = loc.T("flash." + fk)
	}
	return p
}

func New(adminUser, adminPass string, st *store.Store, engine *review.Engine, ocrBinary string) *Server {
	return &Server{
		adminUser: adminUser, adminPass: adminPass,
		store: st, engine: engine,
		ocrBinary: ocrBinary,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.auth(s.dashboard))
	mux.HandleFunc("GET /hosts", s.auth(s.hostsList))
	mux.HandleFunc("GET /hosts/new", s.auth(s.hostNew))
	mux.HandleFunc("POST /hosts", s.auth(s.hostCreate))
	mux.HandleFunc("GET /hosts/{id}/edit", s.auth(s.hostEdit))
	mux.HandleFunc("POST /hosts/{id}", s.auth(s.hostUpdate))
	mux.HandleFunc("GET /llm-providers", s.auth(s.llmProvidersList))
	mux.HandleFunc("GET /llm-providers/new", s.auth(s.llmProviderNew))
	mux.HandleFunc("POST /llm-providers", s.auth(s.llmProviderCreate))
	mux.HandleFunc("POST /llm-providers/test", s.auth(s.llmProviderTest))
	mux.HandleFunc("GET /llm-providers/{id}/edit", s.auth(s.llmProviderEdit))
	mux.HandleFunc("POST /llm-providers/{id}", s.auth(s.llmProviderUpdate))
	mux.HandleFunc("POST /llm-providers/{id}/test", s.auth(s.llmProviderTest))
	mux.HandleFunc("POST /llm-providers/{id}/delete", s.auth(s.llmProviderDelete))
	mux.HandleFunc("POST /llm-providers/{id}/models", s.auth(s.llmModelCreate))
	mux.HandleFunc("POST /llm-providers/{id}/models/discover", s.auth(s.llmProviderModelsDiscover))
	mux.HandleFunc("POST /llm-providers/{id}/models/bulk-update", s.auth(s.llmModelsBulkUpdate))
	mux.HandleFunc("POST /llm-providers/{id}/models/bulk-delete", s.auth(s.llmModelsBulkDelete))
	mux.HandleFunc("GET /repos", s.auth(s.reposList))
	mux.HandleFunc("GET /repos/new", s.auth(s.repoNew))
	mux.HandleFunc("POST /repos", s.auth(s.repoCreate))
	mux.HandleFunc("GET /repos/{id}/edit", s.auth(s.repoEdit))
	mux.HandleFunc("POST /repos/{id}", s.auth(s.repoUpdate))
	mux.HandleFunc("GET /repos/{id}/runs", s.auth(s.repoRuns))
	mux.HandleFunc("POST /repos/{id}/review", s.auth(s.repoManualReview))
	mux.HandleFunc("GET /runs", s.auth(s.runsList))
	mux.HandleFunc("GET /runs/{id}", s.auth(s.runDetail))
	mux.HandleFunc("GET /settings", s.auth(s.settingsForm))
	mux.HandleFunc("POST /settings", s.auth(s.settingsSave))
	mux.HandleFunc("GET /about", s.auth(s.about))
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(s.adminUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(s.adminPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Review Manager"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repos, _ := s.store.ListRepos(ctx)
	hosts, _ := s.store.ListGitHosts(ctx)
	runs, _ := s.store.ListReviewRuns(ctx, 0, 10)
	render(w, "dashboard", struct {
		page
		RepoCount int
		HostCount int
		Runs      []store.ReviewRun
	}{page: s.page(r, "page.dashboard"), RepoCount: len(repos), HostCount: len(hosts), Runs: runs})
}

func (s *Server) hostsList(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListGitHosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	render(w, "hosts", struct {
		page
		Hosts []store.GitHost
	}{page: s.page(r, "page.hosts"), Hosts: hosts})
}

func (s *Server) hostNew(w http.ResponseWriter, r *http.Request) {
	s.renderHostForm(w, r, store.GitHost{
		Kind: "github", APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, "", "/hosts", "page.new_host", "form.pat_optional", false)
}

func (s *Server) hostCreate(w http.ResponseWriter, r *http.Request) {
	h, pat, err := parseHostForm(r)
	if err != nil {
		s.renderHostForm(w, r, h, err.Error(), "/hosts", "page.new_host", "form.pat_optional", false)
		return
	}
	if _, err := s.store.CreateGitHost(r.Context(), h, pat); err != nil {
		s.renderHostForm(w, r, h, err.Error(), "/hosts", "page.new_host", "form.pat_optional", false)
		return
	}
	http.Redirect(w, r, "/hosts?flash=created", http.StatusSeeOther)
}

func (s *Server) hostEdit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	h, err := s.store.GetGitHost(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	patHint := "form.pat_required"
	if h.HasHostPAT {
		patHint = "form.pat_keep"
	}
	s.renderHostForm(w, r, h, "", fmt.Sprintf("/hosts/%d", id), "page.edit_host", patHint, h.HasHostPAT)
}

func (s *Server) hostUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	h, pat, err := parseHostForm(r)
	if err != nil {
		s.renderHostForm(w, r, h, err.Error(), fmt.Sprintf("/hosts/%d", id), "page.edit_host", "", false)
		return
	}
	h.ID = id
	if err := s.store.UpdateGitHost(r.Context(), h, pat, r.FormValue("clear_pat") == "on"); err != nil {
		s.renderHostForm(w, r, h, err.Error(), fmt.Sprintf("/hosts/%d", id), "page.edit_host", "", false)
		return
	}
	http.Redirect(w, r, "/hosts?flash=updated", http.StatusSeeOther)
}

func (s *Server) renderHostForm(w http.ResponseWriter, r *http.Request, h store.GitHost, errMsg, action, titleKey, patHintKey string, showClear bool) {
	p := s.page(r, titleKey)
	render(w, "host_form", struct {
		page
		Host         store.GitHost
		FormTitle    string
		Action       string
		ErrMsg       string
		PATHint      string
		ShowClearPAT bool
	}{page: p, Host: h, FormTitle: p.Title, Action: action, ErrMsg: errMsg, PATHint: p.L.T(patHintKey), ShowClearPAT: showClear})
}

func (s *Server) reposList(w http.ResponseWriter, r *http.Request) {
	repos, err := s.store.ListRepos(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	render(w, "repos", struct {
		page
		Repos []store.RepoView
	}{page: s.page(r, "page.repos"), Repos: repos})
}

func (s *Server) repoNew(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListGitHosts(r.Context())
	if err != nil || len(hosts) == 0 {
		http.Redirect(w, r, "/hosts/new", http.StatusSeeOther)
		return
	}
	rv := store.RepoView{Repo: store.Repo{GitHostID: hosts[0].ID, DefaultBranch: "main", CommentMode: "inline", Enabled: true}}
	s.renderRepoForm(w, r, rv, hosts, nil, "", "", "/repos", "page.new_repo", false)
}

func (s *Server) repoCreate(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListGitHosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	repo, pat, repoURL, err := parseRepoForm(r, hosts)
	if err != nil {
		s.renderRepoForm(w, r, store.RepoView{Repo: repo}, hosts, nil, s.formErr(r, err), repoURL, "/repos", "page.new_repo", false)
		return
	}
	if _, err := s.store.CreateRepo(r.Context(), repo, pat); err != nil {
		s.renderRepoForm(w, r, store.RepoView{Repo: repo}, hosts, nil, err.Error(), repoURL, "/repos", "page.new_repo", false)
		return
	}
	http.Redirect(w, r, "/repos?flash=created", http.StatusSeeOther)
}

func (s *Server) repoEdit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rv, err := s.store.GetRepo(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	hosts, err := s.store.ListGitHosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderRepoForm(w, r, rv, hosts, nil, "", "", fmt.Sprintf("/repos/%d", id), "page.edit_repo", true)
}

func (s *Server) repoUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	hosts, err := s.store.ListGitHosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	repo, pat, repoURL, err := parseRepoForm(r, hosts)
	if err != nil {
		s.renderRepoForm(w, r, store.RepoView{Repo: repo}, hosts, nil, s.formErr(r, err), repoURL, fmt.Sprintf("/repos/%d", id), "page.edit_repo", true)
		return
	}
	repo.ID = id
	if err := s.store.UpdateRepo(r.Context(), repo, pat, r.FormValue("clear_pat") == "on"); err != nil {
		s.renderRepoForm(w, r, store.RepoView{Repo: repo}, hosts, nil, err.Error(), repoURL, fmt.Sprintf("/repos/%d", id), "page.edit_repo", true)
		return
	}
	http.Redirect(w, r, "/repos?flash=updated", http.StatusSeeOther)
}

func (s *Server) formErr(r *http.Request, err error) string {
	if err == nil {
		return ""
	}
	key := err.Error()
	if strings.HasPrefix(key, "form.repo_url") {
		return s.page(r, "").L.T(key)
	}
	return key
}

func (s *Server) renderRepoForm(w http.ResponseWriter, r *http.Request, repo store.RepoView, hosts []store.GitHost, llmOpts []llmPairOption, errMsg, repoURL, action, titleKey string, showClear bool) {
	poll := ""
	if repo.PollIntervalSeconds != nil {
		poll = strconv.Itoa(*repo.PollIntervalSeconds)
	}
	if repo.ID == 0 && !repo.Enabled {
		repo.Enabled = true
	}
	if llmOpts == nil {
		llmOpts, _ = s.llmPairOptionsWithCurrents(r.Context(), repo.EffectiveLLMRotation())
	}
	if repoURL == "" {
		for _, h := range hosts {
			if h.ID == repo.GitHostID {
				repoURL = FormatRepoURL(h.WebBaseURL, repo.Owner, repo.Name)
				break
			}
		}
	}
	p := s.page(r, titleKey)
	render(w, "repo_form", struct {
		page
		Repo         store.RepoView
		Hosts        []store.GitHost
		LLMRotation  llmRotationWidget
		FormTitle    string
		Action       string
		ErrMsg       string
		RepoURL      string
		PollInterval string
		ShowClearPAT bool
	}{
		page: p, Repo: repo, Hosts: hosts,
		LLMRotation: buildLLMRotationWidget(p.L, "llm_pairs", false, llmOpts, repo.EffectiveLLMRotation()),
		FormTitle:   p.Title, Action: action, ErrMsg: errMsg, RepoURL: repoURL, PollInterval: poll, ShowClearPAT: showClear,
	})
}

func (s *Server) repoRuns(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rv, err := s.store.GetRepo(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	runs, _ := s.store.ListReviewRuns(r.Context(), id, 50)
	render(w, "repo_runs", struct {
		page
		Repo store.RepoView
		Runs []store.ReviewRun
	}{page: s.page(r, "page.runs"), Repo: rv, Runs: runs})
}

func (s *Server) repoManualReview(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	prNumber, _ := strconv.Atoi(r.FormValue("pr_number"))
	if prNumber <= 0 {
		http.Redirect(w, r, fmt.Sprintf("/repos/%d/runs?flash=invalid_pr", id), http.StatusSeeOther)
		return
	}
	err := s.engine.ScheduleReview(r.Context(), review.ScheduleRequest{
		RepoID:      id,
		PRNumber:    prNumber,
		TriggerKind: "manual",
	})
	flash := "queued"
	if err != nil {
		slog.ErrorContext(r.Context(), "manual schedule review failed", "repo_id", id, "pr_number", prNumber, "err", err)
		flash = "queue_failed"
	}
	http.Redirect(w, r, fmt.Sprintf("/repos/%d/runs?flash=%s", id, flash), http.StatusSeeOther)
}

func (s *Server) runsList(w http.ResponseWriter, r *http.Request) {
	runs, _ := s.store.ListReviewRuns(r.Context(), 0, 100)
	render(w, "runs", struct {
		page
		Runs []store.ReviewRun
	}{page: s.page(r, "page.runs"), Runs: runs})
}

func (s *Server) runDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	run, err := s.store.GetReviewRun(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ocrJSON := ""
	var summary ocr.Summary
	if run.OCROutputPath != "" {
		if b, err := os.ReadFile(run.OCROutputPath); err == nil {
			var result ocr.Result
			if err := json.Unmarshal(b, &result); err != nil {
				slog.WarnContext(r.Context(), "failed to parse OCR output for summary", "run_id", run.ID, "err", err)
				var pretty bytes.Buffer
				if json.Indent(&pretty, b, "", "  ") == nil {
					ocrJSON = pretty.String()
				} else {
					ocrJSON = string(b)
				}
			} else {
				summary = result.Summary
				if pretty, err := json.MarshalIndent(&result, "", "  "); err == nil {
					ocrJSON = string(pretty)
				} else {
					var pretty bytes.Buffer
					if json.Indent(&pretty, b, "", "  ") == nil {
						ocrJSON = pretty.String()
					} else {
						ocrJSON = string(b)
					}
				}
			}
		}
	}
	p := s.page(r, "page.runs")
	p.Title = fmt.Sprintf("Run #%d", run.ID)
	render(w, "run_detail", struct {
		page
		Run         store.ReviewRun
		SummaryView summaryView
		OCRJSON     string
	}{page: p, Run: run, SummaryView: buildSummaryView(p.L, summary), OCRJSON: ocrJSON})
}

func (s *Server) settingsForm(w http.ResponseWriter, r *http.Request) {
	gs, err := s.store.GetGlobalSettings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	opts, _ := s.llmPairOptionsWithCurrents(r.Context(), gs.EffectiveLLMRotation())
	p := s.page(r, "page.settings")
	render(w, "settings", struct {
		page
		Settings    store.GlobalSettings
		LLMRotation llmRotationWidget
	}{
		page: p, Settings: gs,
		LLMRotation: buildLLMRotationWidget(p.L, "default_llm_pairs", true, opts, gs.EffectiveLLMRotation()),
	})
}

func (s *Server) settingsSave(w http.ResponseWriter, r *http.Request) {
	gs, err := parseSettingsForm(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := s.store.SaveGlobalSettings(r.Context(), gs); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?flash=saved", http.StatusSeeOther)
}

func parseHostForm(r *http.Request) (store.GitHost, string, error) {
	if err := r.ParseForm(); err != nil {
		return store.GitHost{}, "", err
	}
	h := store.GitHost{
		Name:       strings.TrimSpace(r.FormValue("name")),
		Kind:       strings.TrimSpace(r.FormValue("kind")),
		APIBaseURL: strings.TrimSpace(r.FormValue("api_base_url")),
		WebBaseURL: strings.TrimSpace(r.FormValue("web_base_url")),
	}
	if h.Name == "" || h.APIBaseURL == "" || h.WebBaseURL == "" {
		return h, "", fmt.Errorf("name and URLs are required")
	}
	if h.Kind == "" {
		h.Kind = "github"
	}
	return h, strings.TrimSpace(r.FormValue("host_pat")), nil
}

func parseRepoForm(r *http.Request, hosts []store.GitHost) (store.Repo, string, string, error) {
	if err := r.ParseForm(); err != nil {
		return store.Repo{}, "", "", err
	}
	hostID, _ := strconv.ParseInt(r.FormValue("git_host_id"), 10, 64)
	repoURL := strings.TrimSpace(r.FormValue("repo_url"))
	repo := store.Repo{
		GitHostID:              hostID,
		DefaultBranch:          strings.TrimSpace(r.FormValue("default_branch")),
		TriggerLabel:           strings.TrimSpace(r.FormValue("trigger_label")),
		CommentMode:            strings.TrimSpace(r.FormValue("comment_mode")),
		RemoveLabelAfterReview: r.FormValue("remove_label_after_review") == "on",
		ApproveOnZeroFindings:  r.FormValue("approve_on_zero_findings") == "on",
		Enabled:                r.FormValue("enabled") == "on",
		OCRRule:                strings.TrimSpace(r.FormValue("ocr_rule")),
		OCRRequirement:         strings.TrimSpace(r.FormValue("ocr_requirement")),
	}
	repo.OCRBackgroundFile = strings.TrimSpace(r.FormValue("ocr_background_file"))
	bgFile, err := review.NormalizeReviewBackgroundFilePath(repo.OCRBackgroundFile)
	if err != nil {
		return repo, "", repoURL, err
	}
	repo.OCRBackgroundFile = bgFile
	if lang := strings.TrimSpace(r.FormValue("review_language")); lang != "" {
		repo.ReviewLanguage = store.NormalizeReviewLanguage(lang)
	}
	if repo.DefaultBranch == "" {
		repo.DefaultBranch = "main"
	}
	if repo.CommentMode == "" {
		repo.CommentMode = "inline"
	}
	if hostID == 0 || repo.TriggerLabel == "" {
		return repo, "", repoURL, fmt.Errorf("host and trigger label are required")
	}
	var webBase string
	for _, h := range hosts {
		if h.ID == hostID {
			webBase = h.WebBaseURL
			break
		}
	}
	if webBase == "" {
		return repo, "", repoURL, fmt.Errorf("form.repo_url_host_mismatch")
	}
	owner, name, err := ParseRepoURL(repoURL, webBase)
	if err != nil {
		return repo, "", repoURL, err
	}
	repo.Owner, repo.Name = owner, name
	if v := strings.TrimSpace(r.FormValue("poll_interval_seconds")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return repo, "", repoURL, fmt.Errorf("invalid poll interval")
		}
		repo.PollIntervalSeconds = &n
	}
	pairs, err := parseLLMPairsFields(r.Form["llm_pairs"])
	if err != nil {
		return repo, "", repoURL, err
	}
	repo.LLMRotation = pairs
	return repo, strings.TrimSpace(r.FormValue("repo_pat")), repoURL, nil
}

func parseSettingsForm(r *http.Request) (store.GlobalSettings, error) {
	if err := r.ParseForm(); err != nil {
		return store.GlobalSettings{}, err
	}
	parseInt := func(name string) (int, error) {
		v, err := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
		if err != nil || v <= 0 {
			return 0, fmt.Errorf("invalid %s", name)
		}
		return v, nil
	}
	poll, err := parseInt("poll_interval_seconds")
	if err != nil {
		return store.GlobalSettings{}, err
	}
	minPoll, err := parseInt("min_poll_interval_seconds")
	if err != nil {
		return store.GlobalSettings{}, err
	}
	maxConc, err := parseInt("max_concurrent_reviews")
	if err != nil {
		return store.GlobalSettings{}, err
	}
	retention, err := parseInt("review_run_retention_days")
	if err != nil {
		return store.GlobalSettings{}, err
	}
	if minPoll > poll {
		return store.GlobalSettings{}, fmt.Errorf("min poll interval cannot exceed default poll interval")
	}
	pairs, err := parseLLMPairsFields(r.Form["default_llm_pairs"])
	if err != nil {
		return store.GlobalSettings{}, err
	}
	if len(pairs) == 0 {
		return store.GlobalSettings{}, fmt.Errorf("global llm rotation set cannot be empty")
	}
	gs := store.GlobalSettings{
		PollIntervalSeconds:    poll,
		MinPollIntervalSeconds: minPoll,
		MaxConcurrentReviews:   maxConc,
		ReviewRunRetentionDays: retention,
		UILanguage:             store.NormalizeUILanguage(strings.TrimSpace(r.FormValue("ui_language"))),
		ReviewLanguage:         store.NormalizeReviewLanguage(strings.TrimSpace(r.FormValue("review_language"))),
		DefaultLLMRotation:     pairs,
	}.WithDefaults()
	return gs, nil
}
