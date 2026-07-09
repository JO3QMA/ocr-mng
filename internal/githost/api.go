package githost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

type PullRequest struct {
	Number  int
	Title   string
	Body    string
	BaseRef string
	HeadSHA string
	Labels  []string
}

type ReviewComment struct {
	Path      string
	Line      int
	StartLine int
	Body      string
}

type Client struct {
	apiBase    string
	webBase    string
	authPrefix string // "Bearer " or "token "
	listParam  string // per_page or limit
}

func New(kind, apiBase, webBase string) *Client {
	c := &Client{
		apiBase:    trimSlash(apiBase),
		webBase:    trimSlash(webBase),
		authPrefix: "Bearer ",
		listParam:  "per_page",
	}
	if kind == "gitea" {
		c.authPrefix = "token "
		c.listParam = "limit"
	}
	return c
}

func (c *Client) CloneURL(owner, repo, pat string) string {
	return cloneURL(c.webBase, owner, repo, pat)
}

func (c *Client) ListOpenPullRequests(ctx context.Context, owner, repo string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&%s=100", c.apiBase, owner, repo, c.listParam)
	body, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Base   struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]PullRequest, 0, len(raw))
	for _, pr := range raw {
		labels := make([]string, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, PullRequest{
			Number:  pr.Number,
			Title:   pr.Title,
			Body:    pr.Body,
			BaseRef: pr.Base.Ref,
			HeadSHA: pr.Head.SHA,
			Labels:  labels,
		})
	}
	return out, nil
}

func (c *Client) RemoveLabel(ctx context.Context, owner, repo string, prNumber int, label string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels/%s", c.apiBase, owner, repo, prNumber, label)
	_, err := c.do(ctx, http.MethodDelete, url, nil)
	return err
}

func (c *Client) CreateInlineReview(ctx context.Context, owner, repo string, prNumber int, headSHA, body string, comments []ReviewComment) (string, error) {
	type payloadComment struct {
		Path      string `json:"path"`
		Line      int    `json:"line,omitempty"`
		StartLine int    `json:"start_line,omitempty"`
		Body      string `json:"body"`
	}
	commentsPayload := make([]payloadComment, 0, len(comments))
	for _, cm := range comments {
		line := cm.Line
		if line <= 0 {
			line = cm.StartLine
		}
		commentsPayload = append(commentsPayload, payloadComment{
			Path: cm.Path, Line: line, StartLine: cm.StartLine, Body: cm.Body,
		})
	}
	payload := map[string]any{
		"commit_id": headSHA,
		"body":      body,
		"event":     "COMMENT",
		"comments":  commentsPayload,
	}
	respBody, err := c.do(ctx, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", c.apiBase, owner, repo, prNumber), payload)
	if err != nil {
		return "", err
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	_ = json.Unmarshal(respBody, &out)
	return out.HTMLURL, nil
}

func (c *Client) CreateIssueComment(ctx context.Context, owner, repo string, prNumber int, body string) (string, error) {
	respBody, err := c.do(ctx, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.apiBase, owner, repo, prNumber),
		map[string]string{"body": body})
	if err != nil {
		return "", err
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	_ = json.Unmarshal(respBody, &out)
	return out.HTMLURL, nil
}

func (c *Client) do(ctx context.Context, method, url string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if pat, ok := patFromContext(ctx); ok {
		req.Header.Set("Authorization", c.authPrefix+pat)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("git host API %s %s: %s", method, url, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

type ctxKey string

const patKey ctxKey = "pat"

func WithPAT(ctx context.Context, pat string) context.Context {
	return context.WithValue(ctx, patKey, pat)
}

func patFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(patKey).(string)
	return v, ok && v != ""
}

func HasLabel(labels []string, name string) bool {
	for _, l := range labels {
		if l == name {
			return true
		}
	}
	return false
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func cloneURL(webBase, owner, repo, pat string) string {
	if pat == "" {
		return fmt.Sprintf("%s/%s/%s.git", webBase, owner, repo)
	}
	// ponytail: embed PAT in HTTPS URL for git clone; upgrade path is git credential helper
	return fmt.Sprintf("https://oauth2:%s@%s/%s/%s.git",
		pat, trimHost(webBase), owner, repo)
}

func trimHost(webBase string) string {
	u := trimSlash(webBase)
	if len(u) > 8 && u[:8] == "https://" {
		return u[8:]
	}
	if len(u) > 7 && u[:7] == "http://" {
		return u[7:]
	}
	return u
}
