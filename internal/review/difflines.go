package review

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DiffLineSet maps file paths (repo-root relative) to 1-based line numbers on the
// new side of a git diff hunk (context and added lines).
type DiffLineSet map[string]map[int]struct{}

func (d DiffLineSet) has(path string, line int) bool {
	if d == nil {
		return false
	}
	_, ok := d[path][line]
	return ok
}

// CollectDiffLines runs git diff in worktreeDir for fromRef..toSHA.
func CollectDiffLines(ctx context.Context, worktreeDir, fromRef, toSHA string) (DiffLineSet, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", fromRef+".."+toSHA)
	cmd.Dir = worktreeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return ParseDiffLines(string(out)), nil
}

// ParseDiffLines builds a DiffLineSet from unified diff text.
func ParseDiffLines(diff string) DiffLineSet {
	out := make(DiffLineSet)
	var path string
	var newLine int
	inHunk := false

	for _, raw := range strings.Split(diff, "\n") {
		if strings.HasPrefix(raw, "+++ ") {
			p := strings.TrimSpace(strings.TrimPrefix(raw, "+++ "))
			if p == "/dev/null" {
				path = ""
				inHunk = false
				continue
			}
			path = strings.TrimPrefix(p, "b/")
			inHunk = false
			continue
		}
		if strings.HasPrefix(raw, "@@ ") {
			start, ok := parseNewHunkStart(raw)
			if !ok || path == "" {
				inHunk = false
				continue
			}
			newLine = start
			inHunk = true
			continue
		}
		if !inHunk || path == "" || raw == "" {
			continue
		}
		switch raw[0] {
		case ' ', '+':
			if out[path] == nil {
				out[path] = make(map[int]struct{})
			}
			out[path][newLine] = struct{}{}
			newLine++
		case '-':
			// removed on old side only
		case '\\':
			// "\ No newline at end of file"
		}
	}
	return out
}

func parseNewHunkStart(header string) (int, bool) {
	plus := header[strings.Index(header, "+")+1:]
	plus = strings.TrimPrefix(plus, "+")
	end := strings.IndexAny(plus, " ,@")
	if end >= 0 {
		plus = plus[:end]
	}
	n, err := strconv.Atoi(plus)
	return n, err == nil && n > 0
}

// clampToDiff intersects [start, end] with diff lines. Returns GitHub line,
// optional start_line (0 when single-line), and whether any line matched.
func clampToDiff(path string, start, end int, diff DiffLineSet) (line, startLine int, ok bool) {
	if start < 1 {
		start = end
	}
	if end < 1 {
		end = start
	}
	if start > end {
		start, end = end, start
	}
	var matched []int
	for ln := start; ln <= end; ln++ {
		if diff.has(path, ln) {
			matched = append(matched, ln)
		}
	}
	if len(matched) == 0 {
		return 0, 0, false
	}
	first, last := matched[0], matched[len(matched)-1]
	if first == last {
		return first, 0, true
	}
	return last, first, true
}
