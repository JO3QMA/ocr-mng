package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Summary struct {
	FilesReviewed   *int   `json:"files_reviewed,omitempty"`
	Comments        *int   `json:"comments,omitempty"`
	TotalTokens     *int64 `json:"total_tokens,omitempty"`
	InputTokens     *int64 `json:"input_tokens,omitempty"`
	OutputTokens    *int64 `json:"output_tokens,omitempty"`
	CacheReadTokens *int64 `json:"cache_read_tokens,omitempty"`
	Elapsed         string `json:"elapsed,omitempty"`
	BudgetExceeded  *bool  `json:"budget_exceeded,omitempty"`
}

// Present reports whether any known OCR Review Summary field is set for display.
func (s Summary) Present() bool {
	return s.FilesReviewed != nil ||
		s.Comments != nil ||
		s.TotalTokens != nil ||
		s.InputTokens != nil ||
		s.OutputTokens != nil ||
		s.CacheReadTokens != nil ||
		strings.TrimSpace(s.Elapsed) != "" ||
		(s.BudgetExceeded != nil && *s.BudgetExceeded)
}

type Result struct {
	Comments []Comment `json:"comments"`
	Warnings []string  `json:"warnings"`
	Message  string    `json:"message"`
	Summary  Summary   `json:"summary"`
}

type Comment struct {
	FilePath   string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Content    string `json:"content"`
	Suggestion string `json:"suggestion_code"`
	Category   string `json:"category"`
	Severity   string `json:"severity"`
}

type Runner struct {
	Binary     string
	HomeDir    string // contains .opencodereview/config.json
	ConfigJSON string
}

func (r *Runner) Review(ctx context.Context, repoDir, fromRef, toSHA string, model, rule, requirement string) (Result, []byte, error) {
	cfgPath := filepath.Join(r.HomeDir, ".opencodereview", "config.json")
	if strings.TrimSpace(r.ConfigJSON) != "" && r.ConfigJSON != "{}" {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return Result{}, nil, err
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(r.ConfigJSON), "", "  "); err != nil {
			return Result{}, nil, err
		}
		if err := os.WriteFile(cfgPath, pretty.Bytes(), 0o600); err != nil {
			return Result{}, nil, err
		}
	}
	args := []string{"review", "--repo", repoDir, "--from", fromRef, "--to", toSHA, "--format", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if rule != "" {
		args = append(args, "--rule", rule)
	}
	if requirement != "" {
		args = append(args, "--background", requirement)
	}
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+r.HomeDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	raw := stdout.Bytes()
	var result Result
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &result)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return result, raw, fmt.Errorf("%s", msg)
	}
	return result, raw, nil
}
