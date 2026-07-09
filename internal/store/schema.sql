CREATE TABLE IF NOT EXISTS global_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS git_hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL CHECK (kind IN ('github', 'gitea')),
    api_base_url TEXT NOT NULL,
    web_base_url TEXT NOT NULL,
    host_pat_encrypted TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    git_host_id INTEGER NOT NULL REFERENCES git_hosts(id) ON DELETE CASCADE,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    trigger_label TEXT NOT NULL,
    poll_interval_seconds INTEGER,
    repo_pat_encrypted TEXT,
    comment_mode TEXT NOT NULL DEFAULT 'inline' CHECK (comment_mode IN ('inline', 'comment')),
    remove_label_after_review INTEGER NOT NULL DEFAULT 0,
    approve_on_zero_findings INTEGER NOT NULL DEFAULT 0,
    ocr_model TEXT,
    ocr_rule TEXT,
    ocr_requirement TEXT,
    review_language TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    last_polled_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (git_host_id, owner, name)
);

CREATE TABLE IF NOT EXISTS pr_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    pr_number INTEGER NOT NULL,
    has_trigger_label INTEGER NOT NULL DEFAULT 0,
    last_reviewed_head_sha TEXT,
    last_run_id INTEGER,
    updated_at TEXT NOT NULL,
    UNIQUE (repo_id, pr_number)
);

CREATE TABLE IF NOT EXISTS review_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    pr_number INTEGER NOT NULL,
    head_sha TEXT NOT NULL,
    base_ref TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'success', 'failed')),
    trigger_kind TEXT NOT NULL CHECK (trigger_kind IN ('label', 'manual')),
    error_message TEXT,
    post_warning TEXT,
    comment_url TEXT,
    ocr_output_path TEXT,
    summary_total_count INTEGER,
    started_at TEXT,
    finished_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_review_runs_repo ON review_runs(repo_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_repos_host ON repos(git_host_id);
