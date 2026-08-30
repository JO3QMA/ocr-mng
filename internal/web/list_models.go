package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

const maxModelListPages = 100

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
			return http.ErrUseLastResponse
		}
		if req.URL.Host != via[0].URL.Host || (via[0].URL.Scheme == "https" && req.URL.Scheme != "https") {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

type modelsPage struct {
	IDs     []string
	HasMore bool
	AfterID string
}

// ListModels queries the provider HTTP API for model identifiers.
func ListModels(ctx context.Context, apiBaseURL, protocol, apiKey string) ([]string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		protocol = ocr.InferProtocol(apiBaseURL)
	}
	if !ocr.ValidProtocol(protocol) {
		return nil, fmt.Errorf("invalid protocol %q", protocol)
	}
	seen := map[string]struct{}{}
	var out []string
	after := ""
	for page := 0; page < maxModelListPages; page++ {
		pageResult, err := fetchModelsPage(ctx, apiBaseURL, protocol, apiKey, after)
		if err != nil {
			return nil, err
		}
		for _, id := range pageResult.IDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		if protocol != ocr.ProtocolAnthropic || !pageResult.HasMore || pageResult.AfterID == "" {
			break
		}
		after = pageResult.AfterID
	}
	sort.Strings(out)
	return out, nil
}

func fetchModelsPage(ctx context.Context, apiBaseURL, protocol, apiKey, afterID string) (modelsPage, error) {
	endpoint, err := modelsURL(apiBaseURL, protocol, afterID)
	if err != nil {
		return modelsPage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return modelsPage{}, err
	}
	switch protocol {
	case ocr.ProtocolAnthropic:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return modelsPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return modelsPage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		runes := []rune(msg)
		if len(runes) > 200 {
			msg = string(runes[:200]) + "…"
		} else {
			msg = string(runes)
		}
		if msg == "" {
			msg = resp.Status
		}
		return modelsPage{}, fmt.Errorf("models api: %s", msg)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return modelsPage{}, fmt.Errorf("parse models response: %w", err)
	}
	if _, ok := raw["data"]; !ok {
		return modelsPage{}, fmt.Errorf("parse models response: missing data field")
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		HasMore bool   `json:"has_more"`
		LastID  string `json:"last_id"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return modelsPage{}, fmt.Errorf("parse models response: %w", err)
	}
	var ids []string
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return modelsPage{IDs: ids, HasMore: parsed.HasMore, AfterID: strings.TrimSpace(parsed.LastID)}, nil
}

func modelsURL(apiBaseURL, protocol, afterID string) (string, error) {
	raw := ocr.AbsoluteAPIBaseURL(strings.TrimSpace(apiBaseURL))
	if raw == "" {
		return "", fmt.Errorf("api base url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid api base url")
	}
	path := strings.TrimSuffix(u.Path, "/")
	switch {
	case strings.HasSuffix(path, "/models"):
		// already a models URL
	case strings.HasSuffix(path, "/v1"):
		path += "/models"
	case path == "" || path == "/":
		path = "/v1/models"
	default:
		path += "/models"
	}
	u.Path = path
	u.Fragment = ""
	if protocol == ocr.ProtocolAnthropic {
		q := u.Query()
		q.Set("limit", "1000")
		if afterID != "" {
			q.Set("after_id", afterID)
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}
