package review

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	manualCh   chan manualRequest
	repoLocks  sync.Map // repoID -> *sync.Mutex
}

type manualRequest struct {
	RepoID   int64
	PRNumber int
}

func NewEngine(cfg config.Config, st *store.Store, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	e := &Engine{
		cfg:      cfg,
		store:    st,
		log:      log,
		manualCh: make(chan manualRequest, 32),
	}
	return e
}

func (e *Engine) acquireGlobal(ctx context.Context) bool {
	gs, err := e.store.GetGlobalSettings(ctx)
	max := 2
	if err == nil && gs.MaxConcurrentReviews > 0 {
		max = gs.MaxConcurrentReviews
	}
	for {
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

func (e *Engine) TriggerManual(repoID int64, prNumber int) {
	select {
	case e.manualCh <- manualRequest{RepoID: repoID, PRNumber: prNumber}:
	default:
	}
}

func (e *Engine) Run(ctx context.Context) {
	const interrupted = "interrupted: process restarted while review was in progress"
	if n, err := e.store.FailInterruptedReviewRuns(ctx, interrupted); err != nil {
		e.log.Error("recover interrupted reviews", "err", err)
	} else if n > 0 {
		e.log.Info("marked interrupted reviews as failed", "count", n)
	}
	gitwork.PruneMirrors(ctx, filepath.Join(e.cfg.DataDir, "mirrors"))

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
		case req := <-e.manualCh:
			go e.runManual(ctx, req.RepoID, req.PRNumber)
		}
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
		if err := e.pollRepo(ctx, repo, gs); err != nil {
			e.log.Error("poll repo", "repo", repo.Owner+"/"+repo.Name, "err", err)
		}
		_ = e.store.MarkRepoPolled(ctx, repo.ID, now)
	}
}

func (e *Engine) pollRepo(ctx context.Context, repo store.RepoView, gs store.GlobalSettings) error {
	host, err := e.store.GetGitHost(ctx, repo.GitHostID)
	if err != nil {
		return err
	}
	pat, err := e.store.RepoPAT(ctx, repo.ID)
	if err != nil {
		return err
	}
	client := githost.New(host.Kind, host.APIBaseURL, host.WebBaseURL)
	apiCtx := githost.WithPAT(ctx, pat)
	prs, err := client.ListOpenPullRequests(apiCtx, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		hasLabel := githost.HasLabel(pr.Labels, repo.TriggerLabel)
		snap, err := e.store.GetPRSnapshot(ctx, repo.ID, pr.Number)
		if err != nil {
			return err
		}
		// Always update label presence for next transition detection
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
		// Label transition: off -> on. Mark seen before enqueue so the next poll
		// interval does not start a duplicate review while this one is still running.
		_ = e.store.SavePRSnapshot(ctx, deferSnap)
		baseRef := pr.BaseRef
		if baseRef == "" {
			baseRef = repo.DefaultBranch
		}
		e.enqueueReview(repo, host, client, pat, gs, pr, baseRef, "label")
	}
	return nil
}

func (e *Engine) runManual(ctx context.Context, repoID int64, prNumber int) {
	repo, err := e.store.GetRepo(ctx, repoID)
	if err != nil {
		e.log.Error("manual review repo", "err", err)
		return
	}
	host, err := e.store.GetGitHost(ctx, repo.GitHostID)
	if err != nil {
		e.log.Error("manual review host", "err", err)
		return
	}
	pat, err := e.store.RepoPAT(ctx, repo.ID)
	if err != nil {
		e.log.Error("manual review pat", "err", err)
		return
	}
	client := githost.New(host.Kind, host.APIBaseURL, host.WebBaseURL)
	apiCtx := githost.WithPAT(ctx, pat)
	prs, err := client.ListOpenPullRequests(apiCtx, repo.Owner, repo.Name)
	if err != nil {
		e.log.Error("manual list prs", "err", err)
		return
	}
	var target *githost.PullRequest
	for i := range prs {
		if prs[i].Number == prNumber {
			target = &prs[i]
			break
		}
	}
	if target == nil {
		e.log.Error("manual review pr not found", "pr", prNumber)
		return
	}
	gs, _ := e.store.GetGlobalSettings(ctx)
	baseRef := target.BaseRef
	if baseRef == "" {
		baseRef = repo.DefaultBranch
	}
	e.enqueueReview(repo, host, client, pat, gs, *target, baseRef, "manual")
}

func (e *Engine) enqueueReview(repo store.RepoView, host store.GitHost, client *githost.Client, pat string, gs store.GlobalSettings, pr githost.PullRequest, baseRef, triggerKind string) {
	go func() {
		if !e.acquireGlobal(context.Background()) {
			e.log.Warn("review skipped: concurrency limit", "repo", repo.ID, "pr", pr.Number)
			return
		}
		defer e.releaseGlobal()

		mu := e.repoMutex(repo.ID)
		mu.Lock()
		defer mu.Unlock()

		ctx := context.Background()
		runID, err := e.store.CreateReviewRun(ctx, store.ReviewRun{
			RepoID:      repo.ID,
			PRNumber:    pr.Number,
			HeadSHA:     pr.HeadSHA,
			BaseRef:     baseRef,
			Status:      "pending",
			TriggerKind: triggerKind,
		})
		if err != nil {
			e.log.Error("create review run", "err", err)
			return
		}
		started := time.Now()
		run := store.ReviewRun{
			ID:        runID,
			RepoID:    repo.ID,
			PRNumber:  pr.Number,
			HeadSHA:   pr.HeadSHA,
			BaseRef:   baseRef,
			Status:    "running",
			StartedAt: &started,
		}
		_ = e.store.UpdateReviewRun(ctx, run)

		err = e.executeReview(ctx, repo, host, client, pat, gs, pr, baseRef, &run)
		finished := time.Now()
		run.FinishedAt = &finished
		if err != nil {
			run.Status = "failed"
			run.ErrorMessage = err.Error()
			_ = e.store.UpdateReviewRun(ctx, run)
			// Mark label seen to avoid repoll loop; manual retry remains available.
			_ = e.store.SavePRSnapshot(ctx, store.PRSnapshot{
				RepoID:          repo.ID,
				PRNumber:        pr.Number,
				HasTriggerLabel: true,
			})
			e.log.Error("review failed", "run", runID, "err", err)
			return
		}
		run.Status = "success"
		_ = e.store.UpdateReviewRun(ctx, run)

		runIDCopy := runID
		snap := store.PRSnapshot{
			RepoID:              repo.ID,
			PRNumber:            pr.Number,
			HasTriggerLabel:     !repo.RemoveLabelAfterReview && githost.HasLabel(pr.Labels, repo.TriggerLabel),
			LastReviewedHeadSHA: pr.HeadSHA,
			LastRunID:           &runIDCopy,
		}
		if repo.RemoveLabelAfterReview {
			snap.HasTriggerLabel = false
		}
		_ = e.store.SavePRSnapshot(ctx, snap)
	}()
}

func (e *Engine) executeReview(ctx context.Context, repo store.RepoView, host store.GitHost, client *githost.Client, pat string, gs store.GlobalSettings, pr githost.PullRequest, baseRef string, run *store.ReviewRun) error {
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
	configJSON, err := ocr.ConfigWithLanguage(gs.OCRConfigJSON, reviewLang)
	if err != nil {
		return fmt.Errorf("ocr config: %w", err)
	}
	ocrRunner := ocr.Runner{
		Binary:     e.cfg.OCRBinary,
		HomeDir:    filepath.Join(e.cfg.DataDir, "ocr-home"),
		ConfigJSON: configJSON,
	}
	result, raw, err := ocrRunner.Review(ctx, ws.WorktreeDir, fromRef, pr.HeadSHA, repo.OCRModel, repo.OCRRule, MergeOCRRequirement(reviewLang, repo.OCRRequirement))
	if err != nil {
		return fmt.Errorf("ocr review: %w", err)
	}
	ocrDir := filepath.Join(e.cfg.DataDir, "ocr-output")
	_ = os.MkdirAll(ocrDir, 0o755)
	ocrPath := filepath.Join(ocrDir, fmt.Sprintf("run-%d.json", run.ID))
	_ = os.WriteFile(ocrPath, raw, 0o644)
	run.OCROutputPath = ocrPath
	run.SummaryTotalCount = len(result.Comments)

	apiCtx := githost.WithPAT(ctx, pat)
	commentURL, postErr := e.postResult(apiCtx, client, repo, pr, result, reviewLang)
	if postErr != nil {
		return postErr
	}
	run.CommentURL = commentURL

	if repo.RemoveLabelAfterReview {
		if err := client.RemoveLabel(apiCtx, repo.Owner, repo.Name, pr.Number, repo.TriggerLabel); err != nil {
			return fmt.Errorf("remove label: %w", err)
		}
	}
	return nil
}

func (e *Engine) postResult(ctx context.Context, client *githost.Client, repo store.RepoView, pr githost.PullRequest, result ocr.Result, lang string) (string, error) {
	mode := repo.CommentMode
	if mode == "" {
		mode = "inline"
	}
	if mode == "comment" {
		return client.CreateIssueComment(ctx, repo.Owner, repo.Name, pr.Number, AsSingleComment(result, lang))
	}
	inline, body := ForInline(result, lang)
	url, err := client.CreateInlineReview(ctx, repo.Owner, repo.Name, pr.Number, pr.HeadSHA, body, inline)
	if err != nil {
		return client.CreateIssueComment(ctx, repo.Owner, repo.Name, pr.Number, AsSingleComment(result, lang))
	}
	return url, nil
}

func (e *Engine) repoMutex(repoID int64) *sync.Mutex {
	v, _ := e.repoLocks.LoadOrStore(repoID, &sync.Mutex{})
	return v.(*sync.Mutex)
}
