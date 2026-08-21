package ocr_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestReviewRejectsInvalidJSONOutput(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho 'not json'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	_, raw, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "", "", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse ocr output") {
		t.Fatalf("err: %v", err)
	}
	if string(raw) != "not json\n" {
		t.Fatalf("raw: %q", raw)
	}
}

func TestReviewPrefersCommandErrorOverParseError(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho 'not json' >&2\necho 'not json'\nexit 1\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	_, _, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse ocr output") {
		t.Fatalf("parse error when stdout is not JSON: %v", err)
	}
}

func TestReviewParsesJSONWhenCommandFails(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho '{\"comments\":[],\"warnings\":[{\"type\":\"subtask_error\",\"message\":\"boom\"}]}'\necho fail >&2\nexit 1\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	result, _, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "", "", "")
	if err == nil {
		t.Fatal("expected command error")
	}
	if len(result.Warnings) != 1 || !result.HasReviewWarnings() {
		t.Fatalf("parsed result: %+v err=%v", result, err)
	}
}

func TestReviewWithFakeBinary(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\necho '{\"comments\":[],\"message\":\"ok\"}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	result, raw, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "ok" || len(raw) == 0 {
		t.Fatalf("unexpected result: %+v raw=%q", result, raw)
	}
}

func TestReviewPassesBackground(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\ncase \"$*\" in *--background*) echo '{\"comments\":[],\"message\":\"ok\"}' ;; *) echo 'missing background' >&2; exit 1 ;; esac\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	if _, _, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "", "need path", ""); err != nil {
		t.Fatal(err)
	}
}

func TestReviewPassesProviderAndModel(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\ncase \"$*\" in *--provider*anthropic*--model*claude-x*) echo '{\"comments\":[],\"message\":\"ok\"}' ;; *) echo \"args: $*\" >&2; exit 1 ;; esac\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	if _, _, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "anthropic", "claude-x", "", "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestReviewPassesBackgroundFile(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-ocr")
	script := "#!/bin/sh\ncase \"$*\" in *--background-file*docs/ctx.md*) echo '{\"comments\":[],\"message\":\"ok\"}' ;; *) echo \"args: $*\" >&2; exit 1 ;; esac\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := ocr.Runner{Binary: binary, HomeDir: t.TempDir()}
	if _, _, err := runner.Review(context.Background(), dir, "origin/main", "HEAD", "", "", "", "inline", "docs/ctx.md"); err != nil {
		t.Fatal(err)
	}
}

func TestCommentJSONUsesOCRPathKey(t *testing.T) {
	var result ocr.Result
	raw := `{"comments":[{"path":"packages/backend/src/foo.ts","content":"fix","suggestion_code":"bar","start_line":156,"end_line":159,"category":"style","severity":"low"}]}`
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	c := result.Comments[0]
	if c.FilePath != "packages/backend/src/foo.ts" || c.Suggestion != "bar" || c.StartLine != 156 {
		t.Fatalf("comment: %+v", c)
	}
	if c.Category != "style" || c.Severity != "low" {
		t.Fatalf("meta: %+v", c)
	}
}

func TestSummaryJSONParsing(t *testing.T) {
	var result ocr.Result
	raw := `{"status":"success","summary":{"files_reviewed":16,"comments":6,"total_tokens":1344922,"input_tokens":1269674,"output_tokens":75248,"cache_read_tokens":1077120,"elapsed":"3m55s","budget_exceeded":true,"unknown_field":1},"comments":[]}`
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	s := result.Summary
	if s.FilesReviewed == nil || *s.FilesReviewed != 16 {
		t.Fatalf("files_reviewed: %+v", s.FilesReviewed)
	}
	if s.TotalTokens == nil || *s.TotalTokens != 1344922 {
		t.Fatalf("total_tokens: %+v", s.TotalTokens)
	}
	if s.Elapsed != "3m55s" {
		t.Fatalf("elapsed: %q", s.Elapsed)
	}
	if s.BudgetExceeded == nil || !*s.BudgetExceeded {
		t.Fatalf("budget_exceeded: %+v", s.BudgetExceeded)
	}
	if !s.Present() {
		t.Fatal("expected summary present")
	}
}

func TestSummaryPresentEmpty(t *testing.T) {
	if (ocr.Summary{}).Present() {
		t.Fatal("empty summary should not be present")
	}
	var result ocr.Result
	if err := json.Unmarshal([]byte(`{"summary":{}}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary.Present() {
		t.Fatal("empty summary object should not be present")
	}
	if err := json.Unmarshal([]byte(`{"summary":{"budget_exceeded":false}}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary.Present() {
		t.Fatal("budget_exceeded false alone should not be present")
	}
}

func TestResultFailedStatus(t *testing.T) {
	var result ocr.Result
	raw := `{"status":"failed","comments":null,"warnings":null,"message":"Review failed: 0 finding(s); 6 of 6 selected item(s) failed."}`
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Failed() {
		t.Fatal("status failed should be Failed")
	}
	if result.HasReviewWarnings() {
		t.Fatal("failed is a total failure, not a partial warning run")
	}
	if (ocr.Result{Status: "success"}).Failed() || (ocr.Result{}).Failed() {
		t.Fatal("success and empty status are not Failed")
	}
}

func TestWarningsJSONStringAndObject(t *testing.T) {
	var result ocr.Result
	raw := `{
		"status":"completed_with_errors",
		"message":"Some files could not be reviewed due to errors.",
		"comments":[],
		"warnings":[
			"legacy warning",
			{"file":"internal/store/schema.sql","message":"429 Too Many Requests","type":"subtask_error"}
		]
	}`
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if !result.HasReviewWarnings() {
		t.Fatal("expected review warnings")
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings: %+v", result.Warnings)
	}
	if result.Warnings[0].Display() != "legacy warning" {
		t.Fatalf("legacy: %+v", result.Warnings[0])
	}
	if result.Warnings[1].Display() != "internal/store/schema.sql: 429 Too Many Requests" {
		t.Fatalf("object: %+v", result.Warnings[1])
	}
}

func TestWarningsJSONSkipsEmptyElements(t *testing.T) {
	var empty ocr.Result
	if err := json.Unmarshal([]byte(`{"comments":[],"warnings":["",null,{}]}`), &empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Warnings) != 0 {
		t.Fatalf("empty slots: %+v", empty.Warnings)
	}
	if empty.HasReviewWarnings() {
		t.Fatal("empty warning slots should not fail the run")
	}

	var blank ocr.Result
	if err := json.Unmarshal([]byte(`{"comments":[],"warnings":["   "]}`), &blank); err != nil {
		t.Fatal(err)
	}
	if len(blank.Warnings) != 0 {
		t.Fatalf("whitespace warning: %+v", blank.Warnings)
	}

	var one ocr.Result
	if err := json.Unmarshal([]byte(`{"comments":[],"warnings":["",null,"real"]}`), &one); err != nil {
		t.Fatal(err)
	}
	if len(one.Warnings) != 1 || one.Warnings[0].Message != "real" {
		t.Fatalf("warnings: %+v", one.Warnings)
	}
	if !one.HasReviewWarnings() {
		t.Fatal("non-empty warning should fail the run")
	}

	var typeOnly ocr.Result
	if err := json.Unmarshal([]byte(`{"comments":[],"warnings":[{"type":"subtask_error"}]}`), &typeOnly); err != nil {
		t.Fatal(err)
	}
	if len(typeOnly.Warnings) != 1 || typeOnly.Warnings[0].Display() != "subtask_error" {
		t.Fatalf("type-only warning: %+v", typeOnly.Warnings)
	}
	if !typeOnly.HasReviewWarnings() {
		t.Fatal("type-only warning should fail the run")
	}
}
