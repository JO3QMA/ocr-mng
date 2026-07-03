package web

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
)

type Server struct {
	adminUser string
	adminPass string
	store     *store.Store
	engine    *review.Engine
}

type page struct {
	Title string
	Flash string
}

func New(adminUser, adminPass string, st *store.Store, engine *review.Engine) *Server {
	return &Server{adminUser: adminUser, adminPass: adminPass, store: st, engine: engine}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.auth(s.dashboard))
	mux.HandleFunc("GET /hosts", s.auth(s.hostsList))
	mux.HandleFunc("GET /hosts/new", s.auth(s.hostNew))
	mux.HandleFunc("POST /hosts", s.auth(s.hostCreate))
	mux.HandleFunc("GET /hosts/{id}/edit", s.auth(s.hostEdit))
	mux.HandleFunc("POST /hosts/{id}", s.auth(s.hostUpdate))
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
	}{page: page{Title: "Dashboard"}, RepoCount: len(repos), HostCount: len(hosts), Runs: runs})
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
	}{page: page{Title: "Git Hosts", Flash: r.URL.Query().Get("flash")}, Hosts: hosts})
}

func (s *Server) hostNew(w http.ResponseWriter, r *http.Request) {
	s.renderHostForm(w, store.GitHost{
		Kind: "github", APIBaseURL: "https://api.github.com", WebBaseURL: "https://github.com",
	}, true, "", "/hosts", "New Git Host", "(optional)", false)
}

func (s *Server) hostCreate(w http.ResponseWriter, r *http.Request) {
	h, pat, err := parseHostForm(r)
	if err != nil {
		s.renderHostForm(w, h, true, err.Error(), "/hosts", "New Git Host", "(optional)", false)
		return
	}
	if _, err := s.store.CreateGitHost(r.Context(), h, pat); err != nil {
		s.renderHostForm(w, h, true, err.Error(), "/hosts", "New Git Host", "(optional)", false)
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
	patHint := "(required)"
	if h.HasHostPAT {
		patHint = "(leave blank to keep)"
	}
	s.renderHostForm(w, h, false, "", fmt.Sprintf("/hosts/%d", id), "Edit Git Host", patHint, h.HasHostPAT)
}

func (s *Server) hostUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	h, pat, err := parseHostForm(r)
	if err != nil {
		s.renderHostForm(w, h, false, err.Error(), fmt.Sprintf("/hosts/%d", id), "Edit Git Host", "", false)
		return
	}
	h.ID = id
	if err := s.store.UpdateGitHost(r.Context(), h, pat, r.FormValue("clear_pat") == "on"); err != nil {
		s.renderHostForm(w, h, false, err.Error(), fmt.Sprintf("/hosts/%d", id), "Edit Git Host", "", false)
		return
	}
	http.Redirect(w, r, "/hosts?flash=updated", http.StatusSeeOther)
}

func (s *Server) renderHostForm(w http.ResponseWriter, h store.GitHost, _ bool, errMsg, action, title, patHint string, showClear bool) {
	render(w, "host_form", struct {
		page
		Host         store.GitHost
		FormTitle    string
		Action       string
		ErrMsg       string
		PATHint      string
		ShowClearPAT bool
	}{page: page{Title: title}, Host: h, FormTitle: title, Action: action, ErrMsg: errMsg, PATHint: patHint, ShowClearPAT: showClear})
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
	}{page: page{Title: "Repos", Flash: r.URL.Query().Get("flash")}, Repos: repos})
}

func (s *Server) repoNew(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListGitHosts(r.Context())
	if err != nil || len(hosts) == 0 {
		http.Redirect(w, r, "/hosts/new", http.StatusSeeOther)
		return
	}
	rv := store.RepoView{Repo: store.Repo{GitHostID: hosts[0].ID, DefaultBranch: "main", CommentMode: "inline", Enabled: true}}
	s.renderRepoForm(w, rv, hosts, true, "", "/repos", "New Repo", false)
}

func (s *Server) repoCreate(w http.ResponseWriter, r *http.Request) {
	repo, pat, err := parseRepoForm(r)
	if err != nil {
		hosts, _ := s.store.ListGitHosts(r.Context())
		s.renderRepoForm(w, store.RepoView{Repo: repo}, hosts, true, err.Error(), "/repos", "New Repo", false)
		return
	}
	if _, err := s.store.CreateRepo(r.Context(), repo, pat); err != nil {
		hosts, _ := s.store.ListGitHosts(r.Context())
		s.renderRepoForm(w, store.RepoView{Repo: repo}, hosts, true, err.Error(), "/repos", "New Repo", false)
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
	hosts, _ := s.store.ListGitHosts(r.Context())
	s.renderRepoForm(w, rv, hosts, false, "", fmt.Sprintf("/repos/%d", id), "Edit Repo", true)
}

func (s *Server) repoUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	repo, pat, err := parseRepoForm(r)
	if err != nil {
		hosts, _ := s.store.ListGitHosts(r.Context())
		s.renderRepoForm(w, store.RepoView{Repo: repo}, hosts, false, err.Error(), fmt.Sprintf("/repos/%d", id), "Edit Repo", true)
		return
	}
	repo.ID = id
	if err := s.store.UpdateRepo(r.Context(), repo, pat, r.FormValue("clear_pat") == "on"); err != nil {
		hosts, _ := s.store.ListGitHosts(r.Context())
		s.renderRepoForm(w, store.RepoView{Repo: repo}, hosts, false, err.Error(), fmt.Sprintf("/repos/%d", id), "Edit Repo", true)
		return
	}
	http.Redirect(w, r, "/repos?flash=updated", http.StatusSeeOther)
}

func (s *Server) renderRepoForm(w http.ResponseWriter, repo store.RepoView, hosts []store.GitHost, isNew bool, errMsg, action, title string, showClear bool) {
	poll := ""
	if repo.PollIntervalSeconds != nil {
		poll = strconv.Itoa(*repo.PollIntervalSeconds)
	}
	if isNew {
		repo.Enabled = true
	}
	render(w, "repo_form", struct {
		page
		Repo         store.RepoView
		Hosts        []store.GitHost
		FormTitle    string
		Action       string
		ErrMsg       string
		PollInterval string
		ShowClearPAT bool
	}{page: page{Title: title}, Repo: repo, Hosts: hosts, FormTitle: title, Action: action, ErrMsg: errMsg, PollInterval: poll, ShowClearPAT: showClear})
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
	}{page: page{Title: rv.Owner + "/" + rv.Name, Flash: r.URL.Query().Get("flash")}, Repo: rv, Runs: runs})
}

func (s *Server) repoManualReview(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	prNumber, _ := strconv.Atoi(r.FormValue("pr_number"))
	if prNumber <= 0 {
		http.Redirect(w, r, fmt.Sprintf("/repos/%d/runs?flash=invalid+pr", id), http.StatusSeeOther)
		return
	}
	s.engine.TriggerManual(id, prNumber)
	http.Redirect(w, r, fmt.Sprintf("/repos/%d/runs?flash=queued", id), http.StatusSeeOther)
}

func (s *Server) runsList(w http.ResponseWriter, r *http.Request) {
	runs, _ := s.store.ListReviewRuns(r.Context(), 0, 100)
	render(w, "runs", struct {
		page
		Runs []store.ReviewRun
	}{page: page{Title: "Review Runs"}, Runs: runs})
}

func (s *Server) runDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	run, err := s.store.GetReviewRun(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ocrJSON := ""
	if run.OCROutputPath != "" {
		if b, err := os.ReadFile(run.OCROutputPath); err == nil {
			var pretty bytes.Buffer
			if json.Indent(&pretty, b, "", "  ") == nil {
				ocrJSON = pretty.String()
			} else {
				ocrJSON = string(b)
			}
		}
	}
	render(w, "run_detail", struct {
		page
		Run     store.ReviewRun
		OCRJSON string
	}{page: page{Title: fmt.Sprintf("Run #%d", run.ID)}, Run: run, OCRJSON: ocrJSON})
}

func (s *Server) settingsForm(w http.ResponseWriter, r *http.Request) {
	gs, err := s.store.GetGlobalSettings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	render(w, "settings", struct {
		page
		Settings store.GlobalSettings
	}{page: page{Title: "Settings", Flash: r.URL.Query().Get("flash")}, Settings: gs})
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

func parseRepoForm(r *http.Request) (store.Repo, string, error) {
	if err := r.ParseForm(); err != nil {
		return store.Repo{}, "", err
	}
	hostID, _ := strconv.ParseInt(r.FormValue("git_host_id"), 10, 64)
	repo := store.Repo{
		GitHostID:              hostID,
		Owner:                  strings.TrimSpace(r.FormValue("owner")),
		Name:                   strings.TrimSpace(r.FormValue("name")),
		DefaultBranch:          strings.TrimSpace(r.FormValue("default_branch")),
		TriggerLabel:           strings.TrimSpace(r.FormValue("trigger_label")),
		CommentMode:            strings.TrimSpace(r.FormValue("comment_mode")),
		RemoveLabelAfterReview: r.FormValue("remove_label_after_review") == "on",
		Enabled:                r.FormValue("enabled") == "on",
		OCRModel:               strings.TrimSpace(r.FormValue("ocr_model")),
		OCRRule:                strings.TrimSpace(r.FormValue("ocr_rule")),
		OCRRequirement:         strings.TrimSpace(r.FormValue("ocr_requirement")),
	}
	if repo.DefaultBranch == "" {
		repo.DefaultBranch = "main"
	}
	if repo.CommentMode == "" {
		repo.CommentMode = "inline"
	}
	if repo.Owner == "" || repo.Name == "" || repo.TriggerLabel == "" || hostID == 0 {
		return repo, "", fmt.Errorf("host, owner, name, and trigger label are required")
	}
	if v := strings.TrimSpace(r.FormValue("poll_interval_seconds")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return repo, "", fmt.Errorf("invalid poll interval")
		}
		repo.PollIntervalSeconds = &n
	}
	return repo, strings.TrimSpace(r.FormValue("repo_pat")), nil
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
	ocrJSON := strings.TrimSpace(r.FormValue("ocr_config_json"))
	if ocrJSON == "" {
		ocrJSON = "{}"
	}
	if !json.Valid([]byte(ocrJSON)) {
		return store.GlobalSettings{}, fmt.Errorf("invalid OCR config JSON")
	}
	if minPoll > poll {
		return store.GlobalSettings{}, fmt.Errorf("min poll interval cannot exceed default poll interval")
	}
	return store.GlobalSettings{
		PollIntervalSeconds:    poll,
		MinPollIntervalSeconds: minPoll,
		MaxConcurrentReviews:   maxConc,
		ReviewRunRetentionDays: retention,
		OCRConfigJSON:          ocrJSON,
	}, nil
}
