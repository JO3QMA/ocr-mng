package githost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		apiBase:    strings.TrimRight(apiBase, "/"),
		webBase:    strings.TrimRight(webBase, "/"),
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

func (c *Client) ListOpenPullRequests(ctx context.Context, pat, owner, repo string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&%s=100", c.apiBase, owner, repo, c.listParam)
	body, err := c.do(ctx, pat, http.MethodGet, url, nil)
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

func (c *Client) GetPullRequest(ctx context.Context, pat, owner, repo string, prNumber int) (PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.apiBase, owner, repo, prNumber)
	body, err := c.do(ctx, pat, http.MethodGet, url, nil)
	if err != nil {
		return PullRequest{}, err
	}
	var raw struct {
		State  string `json:"state"`
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
		return PullRequest{}, err
	}
	if raw.State != "open" {
		return PullRequest{}, fmt.Errorf("pull request not open: #%d", prNumber)
	}
	labels := make([]string, 0, len(raw.Labels))
	for _, l := range raw.Labels {
		labels = append(labels, l.Name)
	}
	return PullRequest{
		Number:  raw.Number,
		Title:   raw.Title,
		Body:    raw.Body,
		BaseRef: raw.Base.Ref,
		HeadSHA: raw.Head.SHA,
		Labels:  labels,
	}, nil
}

func (c *Client) RemoveLabel(ctx context.Context, pat, owner, repo string, prNumber int, label string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels/%s", c.apiBase, owner, repo, prNumber, label)
	_, err := c.do(ctx, pat, http.MethodDelete, url, nil)
	return err
}

func (c *Client) CreatePullRequestReview(ctx context.Context, pat, owner, repo string, prNumber int, headSHA, body, event string, comments []ReviewComment) (string, error) {
	type payloadComment struct {
		Path      string `json:"path"`
		Line      int    `json:"line,omitempty"`
		StartLine int    `json:"start_line,omitempty"`
		Body      string `json:"body"`
	}
	commentsPayload := make([]payloadComment, 0, len(comments))
	for _, cm := range comments {
		if cm.Path == "" {
			continue
		}
		line := cm.Line
		if line <= 0 {
			line = cm.StartLine
		}
		commentsPayload = append(commentsPayload, payloadComment{
			Path: cm.Path, Line: line, StartLine: cm.StartLine, Body: cm.Body,
		})
	}
	if event == "" {
		event = "COMMENT"
	}
	payload := map[string]any{
		"commit_id": headSHA,
		"body":      body,
		"event":     event,
		"comments":  commentsPayload,
	}
	respBody, err := c.do(ctx, pat, http.MethodPost,
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

func (c *Client) CreateIssueComment(ctx context.Context, pat, owner, repo string, prNumber int, body string) (string, error) {
	respBody, err := c.do(ctx, pat, http.MethodPost,
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

func (c *Client) do(ctx context.Context, pat, method, url string, payload any) ([]byte, error) {
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
	if pat != "" {
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

func cloneURL(webBase, owner, repo, pat string) string {
	if pat == "" {
		return fmt.Sprintf("%s/%s/%s.git", webBase, owner, repo)
	}
	// ponytail: embed PAT in HTTPS URL for git clone; upgrade path is git credential helper
	return fmt.Sprintf("https://oauth2:%s@%s/%s/%s.git",
		pat, hostOf(webBase), owner, repo)
}

func hostOf(webBase string) string {
	u, err := url.Parse(webBase)
	if err != nil || u.Host == "" {
		return strings.TrimPrefix(strings.TrimPrefix(webBase, "https://"), "http://")
	}
	return u.Host
}
