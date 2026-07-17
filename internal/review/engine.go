package review

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jo3qma/ocr-mng/internal/config"
	"github.com/jo3qma/ocr-mng/internal/gitwork"
	"github.com/jo3qma/ocr-mng/internal/githost"
	"github.com/jo3qma/ocr-mng/internal/ocr"
	"github.com/jo3qma/ocr-mng/internal/store"
)

type Engine struct {
	cfg        config.Config
	store      *store.Store
	log        *slog.Logger
	running    atomic.Int32
	dispatchMu sync.Mutex // single-process only; SQL claim when splitting workers
	repoLocks  sync.Map   // repoID -> *sync.Mutex
}

type ScheduleRequest struct {
	RepoID      int64
	PRNumber    int
	HeadSHA     string
	BaseRef     string
	TriggerKind string
}

func NewEngine(cfg config.Config, st *store.Store, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	return &Engine{
		cfg:   cfg,
		store: st,
		log:   log,
	}
}

func (e *Engine) acquireGlobal(ctx context.Context) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	gs, err := e.store.GetGlobalSettings(ctx)
	max := 2
	if err == nil && gs.MaxConcurrentReviews > 0 {
		max = gs.MaxConcurrentReviews
	}
	for {
		if err := ctx.Err(); err != nil {
			return false
		}
		cur := e.running.Load()
		if int(cur) >= max {
			return false
		}
		if e.running.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}

func (e *Engine) releaseGlobal() {
	e.running.Add(-1)
}

// ScheduleReview persists a pending Review Run, or no-ops when one is already active.
// Note: no max pending depth; monitor SQLite size in ops if the queue backs up.
func (e *Engine) ScheduleReview(ctx context.Context, req ScheduleRequest) error {
	if req.BaseRef == "" {
		repo, err := e.store.GetRepo(ctx, req.RepoID)
		if err != nil {
			return err
		}
		req.BaseRef = repo.DefaultBranch
	}
	if req.HeadSHA == "" && req.TriggerKind == "manual" {
		_, _, _, pr, baseRef, err := e.fetchPR(ctx, req.RepoID, req.PRNumber)
		if err != nil {
			return err
		}
		req.HeadSHA = pr.HeadSHA
		req.BaseRef = baseRef
	}
	_, created, err := e.store.CreatePendingReviewRunIfAbsent(ctx, store.ReviewRun{
		RepoID:      req.RepoID,
		PRNumber:    req.PRNumber,
		HeadSHA:     req.HeadSHA,
		BaseRef:     req.BaseRef,
		Status:      "pending",
		TriggerKind: req.TriggerKind,
	})
	if err != nil {
		return err
	}
	if created {
		e.tryDispatch(ctx)
	}
	return nil
}

func (e *Engine) Run(ctx context.Context) {
	const interrupted = "interrupted: process restarted while review was in progress"
	if n, err := e.store.FailInterruptedReviewRuns(ctx, interrupted); err != nil {
		e.log.Error("recover interrupted reviews", "err", err)
	} else if n > 0 {
		e.log.Info("marked interrupted reviews as failed", "count", n)
	}
	// Prune before dispatch so in-flight run-* homes are not deleted under running reviews.
	gitwork.PruneMirrors(ctx, filepath.Join(e.cfg.DataDir, "mirrors"))
	PruneOrphanOCRHomes(filepath.Join(e.cfg.DataDir, "ocr-home"), e.log)
	e.tryDispatch(ctx)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	retention := time.NewTicker(24 * time.Hour)
	defer retention.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.pollAll(ctx)
		case <-retention.C:
			gs, err := e.store.GetGlobalSettings(ctx)
			if err == nil {
				if n, err := e.store.PurgeOldReviewRuns(ctx, gs.ReviewRunRetentionDays); err == nil && n > 0 {
					e.log.Info("purged old review runs", "count", n)
				}
			}
		}
	}
}

const dispatchTimeout = 30 * time.Second

func (e *Engine) tryDispatch(parent context.Context) {
	e.dispatchMu.Lock()
	defer e.dispatchMu.Unlock()

	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, dispatchTimeout)
	defer cancel()

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if !e.acquireGlobal(ctx) {
			return
		}
		run, ok, err := e.store.ClaimNextPendingReviewRun(ctx)
		if err != nil {
			e.releaseGlobal()
			e.log.Error("claim pending review", "err", err)
			return
		}
		if !ok {
			e.releaseGlobal()
			return
		}
		go e.runReview(run)
	}
}

func (e *Engine) pollAll(ctx context.Context) {
	repos, err := e.store.ListRepos(ctx)
	if err != nil {
		e.log.Error("list repos", "err", err)
		return
	}
	gs, err := e.store.GetGlobalSettings(ctx)
	if err != nil {
		e.log.Error("global settings", "err", err)
		return
	}
	now := time.Now()
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}
		interval := gs.PollIntervalSeconds
		if repo.PollIntervalSeconds != nil {
			interval = *repo.PollIntervalSeconds
		}
		if interval < gs.MinPollIntervalSeconds {
			interval = gs.MinPollIntervalSeconds
		}
		if repo.LastPolledAt != nil && now.Sub(*repo.LastPolledAt) < time.Duration(interval)*time.Second {
			continue
		}
		if err := e.pollRepo(ctx, repo); err != nil {
			e.log.Error("poll repo", "repo", repo.Owner+"/"+repo.Name, "err", err)
		}
		_ = e.store.MarkRepoPolled(ctx, repo.ID, now)
	}
}

func (e *Engine) pollRepo(ctx context.Context, repo store.RepoView) error {
	host, err := e.store.GetGitHost(ctx, repo.GitHostID)
	if err != nil {
		return err
	}
	pat, err := e.store.RepoPAT(ctx, repo.ID)
	if err != nil {
		return err
	}
	client := githost.New(host.Kind, host.APIBaseURL, host.WebBaseURL)
	prs, err := client.ListOpenPullRequests(ctx, pat, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		hasLabel := slices.Contains(pr.Labels, repo.TriggerLabel)
		snap, err := e.store.GetPRSnapshot(ctx, repo.ID, pr.Number)
		if err != nil {
			return err
		}
		deferSnap := snap
		deferSnap.HasTriggerLabel = hasLabel
		if !hasLabel {
			_ = e.store.SavePRSnapshot(ctx, deferSnap)
			continue
		}
		if snap.HasTriggerLabel {
			_ = e.store.SavePRSnapshot(ctx, deferSnap)
			continue
		}
		baseRef := pr.BaseRef
		if baseRef == "" {
			baseRef = repo.DefaultBranch
		}
		if err := e.ScheduleReview(ctx, ScheduleRequest{
			RepoID:      repo.ID,
			PRNumber:    pr.Number,
			HeadSHA:     pr.HeadSHA,
			BaseRef:     baseRef,
			TriggerKind: "label",
		}); err != nil {
			e.log.Error("schedule review", "repo_id", repo.ID, "pr_number", pr.Number, "err", err)
			continue
		}
		_ = e.store.SavePRSnapshot(ctx, deferSnap)
	}
	return nil
}

func (e *Engine) runReview(run store.ReviewRun) {
	defer func() {
		if r := recover(); r != nil {
			ctx := context.Background()
			finished := time.Now()
			run.Status = "failed"
			run.ErrorMessage = fmt.Sprintf("panic: %v", r)
			run.FinishedAt = &finished
			_ = e.store.UpdateReviewRun(ctx, run)
			_ = e.store.SavePRSnapshot(ctx, store.PRSnapshot{
				RepoID:          run.RepoID,
				PRNumber:        run.PRNumber,
				HasTriggerLabel: true,
			})
			e.log.Error("review panic", "run_id", run.ID, "repo_id", run.RepoID, "pr_number", run.PRNumber, "panic", r)
		}
		e.releaseGlobal()
		e.tryDispatch(context.Background())
	}()

	ctx := context.Background()
	mu := e.repoMutex(run.RepoID)
	mu.Lock()
	defer mu.Unlock()

	repo, client, pat, pr, baseRef, err := e.fetchPR(ctx, run.RepoID, run.PRNumber)
	if err != nil {
		e.finishFailed(ctx, run, repo, pr, err)
		return
	}

	run.HeadSHA = pr.HeadSHA
	run.BaseRef = baseRef
	_ = e.store.UpdateReviewRun(ctx, run)

	gs, err := e.store.GetGlobalSettings(ctx)
	if err != nil {
		e.finishFailed(ctx, run, repo, pr, err)
		return
	}

	err = e.executeReview(ctx, repo, client, pat, gs, pr, baseRef, &run)
	finished := time.Now()
	run.FinishedAt = &finished
	if err != nil {
		run.Status = "failed"
		run.ErrorMessage = err.Error()
		_ = e.store.UpdateReviewRun(ctx, run)
		_ = e.store.SavePRSnapshot(ctx, store.PRSnapshot{
			RepoID:          repo.ID,
			PRNumber:        pr.Number,
			HasTriggerLabel: true,
		})
		e.log.Error("review failed", "run_id", run.ID, "repo_id", repo.ID, "pr_number", pr.Number, "err", err)
		return
	}
	run.Status = "success"
	_ = e.store.UpdateReviewRun(ctx, run)

	snap := store.PRSnapshot{
		RepoID:          repo.ID,
		PRNumber:        pr.Number,
		HasTriggerLabel: !repo.RemoveLabelAfterReview && slices.Contains(pr.Labels, repo.TriggerLabel),
	}
	if repo.RemoveLabelAfterReview {
		snap.HasTriggerLabel = false
	}
	_ = e.store.SavePRSnapshot(ctx, snap)
}

func (e *Engine) finishFailed(ctx context.Context, run store.ReviewRun, repo store.RepoView, pr githost.PullRequest, err error) {
	finished := time.Now()
	run.Status = "failed"
	run.ErrorMessage = err.Error()
	run.FinishedAt = &finished
	_ = e.store.UpdateReviewRun(ctx, run)
	if repo.ID != 0 {
		_ = e.store.SavePRSnapshot(ctx, store.PRSnapshot{
			RepoID:          repo.ID,
			PRNumber:        run.PRNumber,
			HasTriggerLabel: true,
		})
	}
	e.log.Error("review failed", "run_id", run.ID, "repo_id", run.RepoID, "pr_number", run.PRNumber, "err", err)
}

func (e *Engine) fetchPR(ctx context.Context, repoID int64, prNumber int) (store.RepoView, *githost.Client, string, githost.PullRequest, string, error) {
	repo, err := e.store.GetRepo(ctx, repoID)
	if err != nil {
		return store.RepoView{}, nil, "", githost.PullRequest{}, "", err
	}
	host, err := e.store.GetGitHost(ctx, repo.GitHostID)
	if err != nil {
		return repo, nil, "", githost.PullRequest{}, "", err
	}
	pat, err := e.store.RepoPAT(ctx, repo.ID)
	if err != nil {
		return repo, nil, "", githost.PullRequest{}, "", err
	}
	client := githost.New(host.Kind, host.APIBaseURL, host.WebBaseURL)
	pr, err := client.GetPullRequest(ctx, pat, repo.Owner, repo.Name, prNumber)
	if err != nil {
		return repo, client, pat, githost.PullRequest{}, "", err
	}
	baseRef := pr.BaseRef
	if baseRef == "" {
		baseRef = repo.DefaultBranch
	}
	return repo, client, pat, pr, baseRef, nil
}

func (e *Engine) executeReview(ctx context.Context, repo store.RepoView, client *githost.Client, pat string, gs store.GlobalSettings, pr githost.PullRequest, baseRef string, run *store.ReviewRun) error {
	mirrorsRoot := filepath.Join(e.cfg.DataDir, "mirrors")
	worktreesRoot := filepath.Join(e.cfg.DataDir, "worktrees")
	cloneURL := client.CloneURL(repo.Owner, repo.Name, pat)
	ws, err := gitwork.Prepare(ctx, mirrorsRoot, worktreesRoot, cloneURL, repo.ID, pr.HeadSHA, baseRef)
	if err != nil {
		return err
	}
	defer gitwork.Cleanup(ws)

	fromRef := gitwork.FromRef(baseRef)
	reviewLang := EffectiveReviewLanguage(gs, repo.ReviewLanguage)
	sel, err := ResolveLLMSelection(ctx, e.store, gs, repo, reviewLang)
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}
	run.LLMProviderName = sel.ProviderName
	run.LLMModelName = sel.ModelName

	homeDir := OCRHomeDir(e.cfg.DataDir, run.ID)
	defer func() {
		if err := os.RemoveAll(homeDir); err != nil {
			e.log.Error("ocr home cleanup", "run_id", run.ID, "dir", homeDir, "err", err)
		}
	}()

	ocrRunner := ocr.Runner{
		Binary:     e.cfg.OCRBinary,
		HomeDir:    homeDir,
		ConfigJSON: sel.ConfigJSON,
	}
	result, raw, err := ocrRunner.Review(ctx, ws.WorktreeDir, fromRef, pr.HeadSHA, sel.ModelFlag, repo.OCRRule, BuildReviewBackground(reviewLang, pr.Title, pr.Body, repo.OCRRequirement))
	if err != nil {
		return fmt.Errorf("ocr review: %w", err)
	}
	ocrDir := filepath.Join(e.cfg.DataDir, "ocr-output")
	_ = os.MkdirAll(ocrDir, 0o755)
	ocrPath := filepath.Join(ocrDir, fmt.Sprintf("run-%d.json", run.ID))
	_ = os.WriteFile(ocrPath, raw, 0o644)
	run.OCROutputPath = ocrPath

	commentURL, postErr := e.postResult(ctx, client, pat, repo, pr, result, reviewLang)
	if postErr != nil {
		return postErr
	}
	run.CommentURL = commentURL

	if repo.RemoveLabelAfterReview {
		if err := client.RemoveLabel(ctx, pat, repo.Owner, repo.Name, pr.Number, repo.TriggerLabel); err != nil {
			return fmt.Errorf("remove label: %w", err)
		}
	}
	return nil
}

func (e *Engine) postResult(ctx context.Context, client *githost.Client, pat string, repo store.RepoView, pr githost.PullRequest, result ocr.Result, lang string) (string, error) {
	cf := CommentFormat{Lang: lang, HostKind: repo.HostKind}
	wantApprove := ZeroFindingApprovalEnabled(repo, len(result.Comments))
	mode := repo.CommentMode
	if mode == "" {
		mode = "inline"
	}
	if mode == "comment" {
		if wantApprove {
			if _, err := client.CreatePullRequestReview(ctx, pat, repo.Owner, repo.Name, pr.Number, pr.HeadSHA, ApprovalBody(lang), "APPROVE", nil); err != nil {
				return "", fmt.Errorf("approve review: %w", err)
			}
		}
		return client.CreateIssueComment(ctx, pat, repo.Owner, repo.Name, pr.Number, AsSingleComment(result, cf))
	}
	inline, body := ForInline(result, cf)
	event := "COMMENT"
	if wantApprove {
		event = "APPROVE"
	}
	url, err := client.CreatePullRequestReview(ctx, pat, repo.Owner, repo.Name, pr.Number, pr.HeadSHA, body, event, inline)
	if err != nil {
		if !wantApprove {
			return client.CreateIssueComment(ctx, pat, repo.Owner, repo.Name, pr.Number, AsSingleComment(result, cf))
		}
		if _, err := client.CreatePullRequestReview(ctx, pat, repo.Owner, repo.Name, pr.Number, pr.HeadSHA, ApprovalBody(lang), "APPROVE", nil); err != nil {
			return "", fmt.Errorf("approve review: %w", err)
		}
		return client.CreateIssueComment(ctx, pat, repo.Owner, repo.Name, pr.Number, AsSingleComment(result, cf))
	}
	return url, nil
}

func (e *Engine) repoMutex(repoID int64) *sync.Mutex {
	v, _ := e.repoLocks.LoadOrStore(repoID, &sync.Mutex{})
	return v.(*sync.Mutex)
}
