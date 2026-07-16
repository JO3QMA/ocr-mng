package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jo3qma/ocr-mng/internal/crypto"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db  *sql.DB
	key []byte
}

type GitHost struct {
	ID               int64
	Name             string
	Kind             string
	APIBaseURL       string
	WebBaseURL       string
	HasHostPAT       bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Repo struct {
	ID                     int64
	GitHostID              int64
	Owner                  string
	Name                   string
	DefaultBranch          string
	TriggerLabel           string
	PollIntervalSeconds    *int
	CommentMode            string
	RemoveLabelAfterReview bool
	ApproveOnZeroFindings  bool
	OCRModel               string
	OCRRule                string
	OCRRequirement         string
	ReviewLanguage         string
	Enabled                bool
	LastPolledAt           *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RepoView struct {
	Repo
	HostName string
	HostKind string
}

type PRSnapshot struct {
	ID                   int64
	RepoID               int64
	PRNumber             int
	HasTriggerLabel      bool
	LastReviewedHeadSHA  string
	LastRunID            *int64
	UpdatedAt            time.Time
}

type ReviewRun struct {
	ID                  int64
	RepoID              int64
	PRNumber            int
	HeadSHA             string
	BaseRef             string
	Status              string
	TriggerKind         string
	ErrorMessage        string
	CommentURL          string
	OCROutputPath       string
	StartedAt           *time.Time
	FinishedAt          *time.Time
	CreatedAt           time.Time
}

type GlobalSettings struct {
	PollIntervalSeconds    int `json:"poll_interval_seconds"`
	MinPollIntervalSeconds int `json:"min_poll_interval_seconds"`
	MaxConcurrentReviews   int `json:"max_concurrent_reviews"`
	ReviewRunRetentionDays int    `json:"review_run_retention_days"`
	OCRConfigJSON          string `json:"ocr_config_json"`
	UILanguage             string `json:"ui_language"`
	ReviewLanguage         string `json:"review_language"`
}

func NormalizeUILanguage(s string) string {
	if s == "en" {
		return "en"
	}
	return "ja"
}

func NormalizeReviewLanguage(s string) string {
	switch s {
	case "English", "Chinese":
		return s
	default:
		return "Japanese"
	}
}

func (gs GlobalSettings) WithDefaults() GlobalSettings {
	if gs.PollIntervalSeconds < 1 {
		gs.PollIntervalSeconds = 300
	}
	if gs.MinPollIntervalSeconds < 1 {
		gs.MinPollIntervalSeconds = 120
	}
	if gs.MaxConcurrentReviews < 1 {
		gs.MaxConcurrentReviews = 2
	}
	if gs.ReviewRunRetentionDays < 1 {
		gs.ReviewRunRetentionDays = 30
	}
	if strings.TrimSpace(gs.OCRConfigJSON) == "" {
		gs.OCRConfigJSON = "{}"
	}
	gs.UILanguage = NormalizeUILanguage(gs.UILanguage)
	gs.ReviewLanguage = NormalizeReviewLanguage(gs.ReviewLanguage)
	return gs
}

func Open(path string, key []byte) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db, key: key}
	if err := s.ensureDefaults(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) ensureDefaults(ctx context.Context) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM global_settings`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return s.migrate(ctx)
	}
	gs := GlobalSettings{
		PollIntervalSeconds:    300,
		MinPollIntervalSeconds: 120,
		MaxConcurrentReviews:   2,
		ReviewRunRetentionDays: 30,
		OCRConfigJSON:          `{}`,
		UILanguage:             "ja",
		ReviewLanguage:         "Japanese",
	}.WithDefaults()
	b, _ := json.Marshal(gs)
	_, err := s.db.ExecContext(ctx, `INSERT INTO global_settings(key, value) VALUES ('settings', ?)`, string(b))
	return err
}

func (s *Store) migrate(ctx context.Context) error {
	for _, stmt := range []string{
		`ALTER TABLE repos ADD COLUMN review_language TEXT`,
		`ALTER TABLE repos ADD COLUMN approve_on_zero_findings INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE review_runs DROP COLUMN summary_total_count`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "no such column") {
				continue
			}
			return fmt.Errorf("migration %q: %w", stmt, err)
		}
	}
	return nil
}

func (s *Store) GetGlobalSettings(ctx context.Context) (GlobalSettings, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM global_settings WHERE key = 'settings'`).Scan(&raw)
	if err != nil {
		return GlobalSettings{}, err
	}
	var gs GlobalSettings
	if err := json.Unmarshal([]byte(raw), &gs); err != nil {
		return GlobalSettings{}, err
	}
	return gs.WithDefaults(), nil
}

func (s *Store) SaveGlobalSettings(ctx context.Context, gs GlobalSettings) error {
	b, err := json.Marshal(gs)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE global_settings SET value = ? WHERE key = 'settings'`, string(b))
	return err
}

func (s *Store) encryptPAT(pat string) (string, error) {
	if pat == "" {
		return "", nil
	}
	return crypto.Encrypt(s.key, []byte(pat))
}

func (s *Store) decryptPAT(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	b, err := crypto.Decrypt(s.key, enc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Store) RepoPAT(ctx context.Context, repoID int64) (string, error) {
	var repoEnc, hostEnc sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT r.repo_pat_encrypted, h.host_pat_encrypted
		FROM repos r JOIN git_hosts h ON h.id = r.git_host_id
		WHERE r.id = ?`, repoID).Scan(&repoEnc, &hostEnc)
	if err != nil {
		return "", err
	}
	if repoEnc.Valid && repoEnc.String != "" {
		return s.decryptPAT(repoEnc.String)
	}
	if hostEnc.Valid && hostEnc.String != "" {
		return s.decryptPAT(hostEnc.String)
	}
	return "", fmt.Errorf("no PAT configured for repo %d", repoID)
}

func (s *Store) CreateGitHost(ctx context.Context, h GitHost, pat string) (int64, error) {
	enc, err := s.encryptPAT(pat)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO git_hosts(name, kind, api_base_url, web_base_url, host_pat_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		h.Name, h.Kind, h.APIBaseURL, h.WebBaseURL, nullIfEmpty(enc), now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateGitHost(ctx context.Context, h GitHost, pat string, clearPAT bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if pat != "" {
		enc, err := s.encryptPAT(pat)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE git_hosts SET name=?, kind=?, api_base_url=?, web_base_url=?, host_pat_encrypted=?, updated_at=?
			WHERE id=?`, h.Name, h.Kind, h.APIBaseURL, h.WebBaseURL, enc, now, h.ID)
		return err
	}
	if clearPAT {
		_, err := s.db.ExecContext(ctx, `
			UPDATE git_hosts SET name=?, kind=?, api_base_url=?, web_base_url=?, host_pat_encrypted=NULL, updated_at=?
			WHERE id=?`, h.Name, h.Kind, h.APIBaseURL, h.WebBaseURL, now, h.ID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE git_hosts SET name=?, kind=?, api_base_url=?, web_base_url=?, updated_at=?
		WHERE id=?`, h.Name, h.Kind, h.APIBaseURL, h.WebBaseURL, now, h.ID)
	return err
}

func (s *Store) ListGitHosts(ctx context.Context) ([]GitHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, kind, api_base_url, web_base_url,
		       CASE WHEN host_pat_encrypted IS NOT NULL AND host_pat_encrypted != '' THEN 1 ELSE 0 END,
		       created_at, updated_at FROM git_hosts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []GitHost
	for rows.Next() {
		var h GitHost
		var created, updated string
		var has int
		if err := rows.Scan(&h.ID, &h.Name, &h.Kind, &h.APIBaseURL, &h.WebBaseURL, &has, &created, &updated); err != nil {
			return nil, err
		}
		h.HasHostPAT = has == 1
		h.CreatedAt, _ = time.Parse(time.RFC3339, created)
		h.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) GetGitHost(ctx context.Context, id int64) (GitHost, error) {
	var h GitHost
	var created, updated string
	var has int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, kind, api_base_url, web_base_url,
		       CASE WHEN host_pat_encrypted IS NOT NULL AND host_pat_encrypted != '' THEN 1 ELSE 0 END,
		       created_at, updated_at FROM git_hosts WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.Kind, &h.APIBaseURL, &h.WebBaseURL, &has, &created, &updated)
	if err != nil {
		return GitHost{}, err
	}
	h.HasHostPAT = has == 1
	h.CreatedAt, _ = time.Parse(time.RFC3339, created)
	h.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return h, nil
}

func (s *Store) CreateRepo(ctx context.Context, r Repo, pat string) (int64, error) {
	enc, err := s.encryptPAT(pat)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	remove := 0
	if r.RemoveLabelAfterReview {
		remove = 1
	}
	approve := 0
	if r.ApproveOnZeroFindings {
		approve = 1
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO repos(git_host_id, owner, name, default_branch, trigger_label, poll_interval_seconds,
			repo_pat_encrypted, comment_mode, remove_label_after_review, approve_on_zero_findings,
			ocr_model, ocr_rule, ocr_requirement, review_language, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.GitHostID, r.Owner, r.Name, r.DefaultBranch, r.TriggerLabel, r.PollIntervalSeconds,
		nullIfEmpty(enc), r.CommentMode, remove, approve, nullStr(r.OCRModel), nullStr(r.OCRRule), nullStr(r.OCRRequirement),
		nullStr(r.ReviewLanguage), enabled, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateRepo(ctx context.Context, r Repo, pat string, clearPAT bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	remove := 0
	if r.RemoveLabelAfterReview {
		remove = 1
	}
	approve := 0
	if r.ApproveOnZeroFindings {
		approve = 1
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	if pat != "" {
		enc, err := s.encryptPAT(pat)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE repos SET git_host_id=?, owner=?, name=?, default_branch=?, trigger_label=?,
				poll_interval_seconds=?, repo_pat_encrypted=?, comment_mode=?, remove_label_after_review=?,
				approve_on_zero_findings=?, ocr_model=?, ocr_rule=?, ocr_requirement=?, review_language=?,
				enabled=?, updated_at=?
			WHERE id=?`,
			r.GitHostID, r.Owner, r.Name, r.DefaultBranch, r.TriggerLabel, r.PollIntervalSeconds, enc,
			r.CommentMode, remove, approve, nullStr(r.OCRModel), nullStr(r.OCRRule), nullStr(r.OCRRequirement),
			nullStr(r.ReviewLanguage), enabled, now, r.ID)
		return err
	}
	if clearPAT {
		_, err := s.db.ExecContext(ctx, `
			UPDATE repos SET git_host_id=?, owner=?, name=?, default_branch=?, trigger_label=?,
				poll_interval_seconds=?, repo_pat_encrypted=NULL, comment_mode=?, remove_label_after_review=?,
				approve_on_zero_findings=?, ocr_model=?, ocr_rule=?, ocr_requirement=?, review_language=?,
				enabled=?, updated_at=?
			WHERE id=?`,
			r.GitHostID, r.Owner, r.Name, r.DefaultBranch, r.TriggerLabel, r.PollIntervalSeconds,
			r.CommentMode, remove, approve, nullStr(r.OCRModel), nullStr(r.OCRRule), nullStr(r.OCRRequirement),
			nullStr(r.ReviewLanguage), enabled, now, r.ID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE repos SET git_host_id=?, owner=?, name=?, default_branch=?, trigger_label=?,
			poll_interval_seconds=?, comment_mode=?, remove_label_after_review=?, approve_on_zero_findings=?,
			ocr_model=?, ocr_rule=?, ocr_requirement=?, review_language=?, enabled=?, updated_at=?
		WHERE id=?`,
		r.GitHostID, r.Owner, r.Name, r.DefaultBranch, r.TriggerLabel, r.PollIntervalSeconds,
		r.CommentMode, remove, approve, nullStr(r.OCRModel), nullStr(r.OCRRule), nullStr(r.OCRRequirement),
		nullStr(r.ReviewLanguage), enabled, now, r.ID)
	return err
}

func scanRepo(scanner interface {
	Scan(dest ...any) error
}) (RepoView, error) {
	var rv RepoView
	var poll sql.NullInt64
	var lastPolled sql.NullString
	var remove, approve, enabled int
	var ocrModel, ocrRule, ocrReq, reviewLang sql.NullString
	var created, updated string
	err := scanner.Scan(
		&rv.ID, &rv.GitHostID, &rv.Owner, &rv.Name, &rv.DefaultBranch, &rv.TriggerLabel, &poll,
		&rv.CommentMode, &remove, &approve, &ocrModel, &ocrRule, &ocrReq, &reviewLang, &enabled, &lastPolled, &created, &updated,
		&rv.HostName, &rv.HostKind,
	)
	if err != nil {
		return RepoView{}, err
	}
	if poll.Valid {
		v := int(poll.Int64)
		rv.PollIntervalSeconds = &v
	}
	rv.RemoveLabelAfterReview = remove == 1
	rv.ApproveOnZeroFindings = approve == 1
	rv.Enabled = enabled == 1
	if ocrModel.Valid {
		rv.OCRModel = ocrModel.String
	}
	if ocrRule.Valid {
		rv.OCRRule = ocrRule.String
	}
	if ocrReq.Valid {
		rv.OCRRequirement = ocrReq.String
	}
	if reviewLang.Valid {
		rv.ReviewLanguage = reviewLang.String
	}
	if lastPolled.Valid {
		t, _ := time.Parse(time.RFC3339, lastPolled.String)
		rv.LastPolledAt = &t
	}
	rv.CreatedAt, _ = time.Parse(time.RFC3339, created)
	rv.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return rv, nil
}

func (s *Store) ListRepos(ctx context.Context) ([]RepoView, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.git_host_id, r.owner, r.name, r.default_branch, r.trigger_label, r.poll_interval_seconds,
			r.comment_mode, r.remove_label_after_review, r.approve_on_zero_findings, r.ocr_model, r.ocr_rule, r.ocr_requirement, r.review_language,
			r.enabled, r.last_polled_at, r.created_at, r.updated_at, h.name, h.kind
		FROM repos r JOIN git_hosts h ON h.id = r.git_host_id
		ORDER BY h.name, r.owner, r.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RepoView
	for rows.Next() {
		rv, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

func (s *Store) GetRepo(ctx context.Context, id int64) (RepoView, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT r.id, r.git_host_id, r.owner, r.name, r.default_branch, r.trigger_label, r.poll_interval_seconds,
			r.comment_mode, r.remove_label_after_review, r.approve_on_zero_findings, r.ocr_model, r.ocr_rule, r.ocr_requirement, r.review_language,
			r.enabled, r.last_polled_at, r.created_at, r.updated_at, h.name, h.kind
		FROM repos r JOIN git_hosts h ON h.id = r.git_host_id
		WHERE r.id=?`, id)
	return scanRepo(row)
}

func (s *Store) MarkRepoPolled(ctx context.Context, repoID int64, t time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE repos SET last_polled_at=? WHERE id=?`, t.UTC().Format(time.RFC3339), repoID)
	return err
}

func (s *Store) GetPRSnapshot(ctx context.Context, repoID int64, prNumber int) (PRSnapshot, error) {
	var snap PRSnapshot
	var has int
	var lastSHA sql.NullString
	var lastRun sql.NullInt64
	var updated string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, repo_id, pr_number, has_trigger_label, last_reviewed_head_sha, last_run_id, updated_at
		FROM pr_snapshots WHERE repo_id=? AND pr_number=?`, repoID, prNumber).
		Scan(&snap.ID, &snap.RepoID, &snap.PRNumber, &has, &lastSHA, &lastRun, &updated)
	if err == sql.ErrNoRows {
		return PRSnapshot{RepoID: repoID, PRNumber: prNumber}, nil
	}
	if err != nil {
		return PRSnapshot{}, err
	}
	snap.HasTriggerLabel = has == 1
	if lastSHA.Valid {
		snap.LastReviewedHeadSHA = lastSHA.String
	}
	if lastRun.Valid {
		v := lastRun.Int64
		snap.LastRunID = &v
	}
	snap.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return snap, nil
}

func (s *Store) SavePRSnapshot(ctx context.Context, snap PRSnapshot) error {
	has := 0
	if snap.HasTriggerLabel {
		has = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pr_snapshots(repo_id, pr_number, has_trigger_label, last_reviewed_head_sha, last_run_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, pr_number) DO UPDATE SET
			has_trigger_label=excluded.has_trigger_label,
			last_reviewed_head_sha=excluded.last_reviewed_head_sha,
			last_run_id=excluded.last_run_id,
			updated_at=excluded.updated_at`,
		snap.RepoID, snap.PRNumber, has, nullStr(snap.LastReviewedHeadSHA), snap.LastRunID, now)
	return err
}

func (s *Store) CreateReviewRun(ctx context.Context, run ReviewRun) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO review_runs(repo_id, pr_number, head_sha, base_ref, status, trigger_kind, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.RepoID, run.PRNumber, run.HeadSHA, run.BaseRef, run.Status, run.TriggerKind, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateReviewRun(ctx context.Context, run ReviewRun) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE review_runs SET status=?, error_message=?, comment_url=?, ocr_output_path=?,
			started_at=?, finished_at=?
		WHERE id=?`,
		run.Status, nullStr(run.ErrorMessage), nullStr(run.CommentURL), nullStr(run.OCROutputPath),
		formatTime(run.StartedAt), formatTime(run.FinishedAt), run.ID)
	return err
}

func (s *Store) ListReviewRuns(ctx context.Context, repoID int64, limit int) ([]ReviewRun, error) {
	q := `SELECT id, repo_id, pr_number, head_sha, base_ref, status, trigger_kind, error_message,
		comment_url, ocr_output_path, started_at, finished_at, created_at
		FROM review_runs`
	var rows *sql.Rows
	var err error
	if repoID > 0 {
		q += ` WHERE repo_id=? ORDER BY id DESC LIMIT ?`
		rows, err = s.db.QueryContext(ctx, q, repoID, limit)
	} else {
		q += ` ORDER BY id DESC LIMIT ?`
		rows, err = s.db.QueryContext(ctx, q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanReviewRuns(rows)
}

func (s *Store) GetReviewRun(ctx context.Context, id int64) (ReviewRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, repo_id, pr_number, head_sha, base_ref, status, trigger_kind, error_message,
			comment_url, ocr_output_path, started_at, finished_at, created_at
		FROM review_runs WHERE id=?`, id)
	return scanReviewRun(row)
}

func scanReviewRuns(rows *sql.Rows) ([]ReviewRun, error) {
	var out []ReviewRun
	for rows.Next() {
		r, err := scanReviewRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanReviewRun(scanner interface {
	Scan(dest ...any) error
}) (ReviewRun, error) {
	var r ReviewRun
	var errMsg, commentURL, ocrPath sql.NullString
	var started, finished, created sql.NullString
	err := scanner.Scan(
		&r.ID, &r.RepoID, &r.PRNumber, &r.HeadSHA, &r.BaseRef, &r.Status, &r.TriggerKind,
		&errMsg, &commentURL, &ocrPath,
		&started, &finished, &created,
	)
	if err != nil {
		return ReviewRun{}, err
	}
	if errMsg.Valid {
		r.ErrorMessage = errMsg.String
	}
	if commentURL.Valid {
		r.CommentURL = commentURL.String
	}
	if ocrPath.Valid {
		r.OCROutputPath = ocrPath.String
	}
	r.StartedAt = parseTime(started)
	r.FinishedAt = parseTime(finished)
	if created.Valid {
		t, _ := time.Parse(time.RFC3339, created.String)
		r.CreatedAt = t
	}
	return r, nil
}

func (s *Store) FailInterruptedReviewRuns(ctx context.Context, reason string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE review_runs
		SET status='failed', error_message=?, finished_at=?
		WHERE status IN ('pending', 'running')`, reason, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) PurgeOldReviewRuns(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `DELETE FROM review_runs WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(v sql.NullString) *time.Time {
	if !v.Valid || v.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v.String)
	if err != nil {
		return nil
	}
	return &t
}
