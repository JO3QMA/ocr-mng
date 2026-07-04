package gitwork

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Workspace struct {
	MirrorDir   string
	WorktreeDir string
}

func Prepare(ctx context.Context, mirrorsRoot, worktreesRoot, cloneURL string, repoID int64, headSHA, baseRef string) (Workspace, error) {
	mirrorDir := filepath.Join(mirrorsRoot, strconv.FormatInt(repoID, 10))
	if err := os.MkdirAll(mirrorsRoot, 0o755); err != nil {
		return Workspace{}, err
	}
	if _, err := os.Stat(filepath.Join(mirrorDir, "HEAD")); os.IsNotExist(err) {
		if err := runGit(ctx, "", "clone", "--mirror", cloneURL, mirrorDir); err != nil {
			return Workspace{}, fmt.Errorf("mirror clone: %w", err)
		}
	} else if err != nil {
		return Workspace{}, err
	} else {
		if err := runGit(ctx, mirrorDir, "remote", "set-url", "origin", cloneURL); err != nil {
			return Workspace{}, err
		}
		if err := runGit(ctx, mirrorDir, "fetch", "--prune", "origin"); err != nil {
			return Workspace{}, fmt.Errorf("mirror fetch: %w", err)
		}
	}

	worktreeDir, err := filepath.Abs(filepath.Join(worktreesRoot, fmt.Sprintf("%d-%s", repoID, headSHA[:8])))
	if err != nil {
		return Workspace{}, err
	}
	clearWorktree(ctx, mirrorDir, worktreeDir)
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		return Workspace{}, err
	}
	if err := runGit(ctx, mirrorDir, "worktree", "add", "--detach", worktreeDir, headSHA); err != nil {
		// ponytail: crashed reviews can leave a registered-but-missing worktree; -f recovers.
		clearWorktree(ctx, mirrorDir, worktreeDir)
		if err2 := runGit(ctx, mirrorDir, "worktree", "add", "--detach", "-f", worktreeDir, headSHA); err2 != nil {
			return Workspace{}, fmt.Errorf("worktree add (retry after clear): %w (original: %v)", err2, err)
		}
	}
	// Ensure base ref exists locally for --from origin/<base>
	_ = runGit(ctx, mirrorDir, "fetch", "origin", baseRef+":"+"refs/remotes/origin/"+baseRef)

	return Workspace{MirrorDir: mirrorDir, WorktreeDir: worktreeDir}, nil
}

func Cleanup(ws Workspace) {
	if ws.WorktreeDir != "" {
		clearWorktree(context.Background(), ws.MirrorDir, ws.WorktreeDir)
	}
}

func clearWorktree(ctx context.Context, mirrorDir, worktreeDir string) {
	_ = runGit(ctx, mirrorDir, "worktree", "remove", "--force", worktreeDir)
	_ = os.RemoveAll(worktreeDir)
	_ = runGit(ctx, mirrorDir, "worktree", "prune")
}

// PruneMirrors drops stale worktree registrations left by interrupted reviews.
func PruneMirrors(ctx context.Context, mirrorsRoot string) {
	entries, err := os.ReadDir(mirrorsRoot)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if ent.IsDir() {
			_ = runGit(ctx, filepath.Join(mirrorsRoot, ent.Name()), "worktree", "prune")
		}
	}
}

func FromRef(baseRef string) string {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		return "origin/main"
	}
	if strings.HasPrefix(baseRef, "origin/") {
		return baseRef
	}
	return "origin/" + baseRef
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
