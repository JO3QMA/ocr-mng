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

// Warning is one OCR review warning (string legacy or object with file/message/type).
type Warning struct {
	File    string `json:"file"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// Display formats a warning for PR comment bullet lines.
func (w Warning) Display() string {
	msg := strings.TrimSpace(w.Message)
	file := strings.TrimSpace(w.File)
	if file != "" && msg != "" {
		return file + ": " + msg
	}
	if msg != "" {
		return msg
	}
	return file
}

// Warnings unmarshals OCR warnings as either strings or objects.
type Warnings []Warning

func (w *Warnings) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*w = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(Warnings, 0, len(raw))
	for _, item := range raw {
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			out = append(out, Warning{Message: s})
			continue
		}
		var obj Warning
		if err := json.Unmarshal(item, &obj); err != nil {
			return err
		}
		out = append(out, obj)
	}
	*w = out
	return nil
}

type Result struct {
	Status   string    `json:"status"`
	Comments []Comment `json:"comments"`
	Warnings Warnings  `json:"warnings"`
	Message  string    `json:"message"`
	Summary  Summary   `json:"summary"`
}

// HasReviewWarnings reports incomplete OCR runs that should fail and keep the trigger label.
func (r Result) HasReviewWarnings() bool {
	if len(r.Warnings) > 0 {
		return true
	}
	return r.Status == "completed_with_errors"
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

func (r *Runner) writeConfig() error {
	if strings.TrimSpace(r.ConfigJSON) == "" || r.ConfigJSON == "{}" {
		return nil
	}
	cfgPath := filepath.Join(r.HomeDir, ".opencodereview", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(r.ConfigJSON), "", "  "); err != nil {
		return err
	}
	return os.WriteFile(cfgPath, pretty.Bytes(), 0o600)
}

func (r *Runner) Review(ctx context.Context, repoDir, fromRef, toSHA string, provider, model, rule, requirement, backgroundFile string) (Result, []byte, error) {
	if err := r.writeConfig(); err != nil {
		return Result{}, nil, err
	}
	args := []string{"review", "--repo", repoDir, "--from", fromRef, "--to", toSHA, "--format", "json"}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if rule != "" {
		args = append(args, "--rule", rule)
	}
	if requirement != "" {
		args = append(args, "--background", requirement)
	}
	if backgroundFile != "" {
		args = append(args, "--background-file", backgroundFile)
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

// TestLLM runs `ocr llm test` against ConfigJSON in an isolated HomeDir.
func (r *Runner) TestLLM(ctx context.Context) error {
	if strings.TrimSpace(r.ConfigJSON) == "" || r.ConfigJSON == "{}" {
		return fmt.Errorf("config is required")
	}
	if err := r.writeConfig(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, r.Binary, "llm", "test")
	cmd.Env = append(os.Environ(), "HOME="+r.HomeDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			if ctx.Err() != nil {
				msg = ctx.Err().Error()
			} else {
				msg = err.Error()
			}
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// MaskSecret replaces every occurrence of secret in msg with *** (no-op if secret empty).
func MaskSecret(msg, secret string) string {
	if secret == "" || msg == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "***")
}
