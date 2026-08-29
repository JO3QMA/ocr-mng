package llmclient

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

var httpClient = &http.Client{Timeout: 30 * time.Second}

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
	endpoint, err := modelsEndpoint(apiBaseURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("models api: %s", msg)
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}
	seen := map[string]struct{}{}
	var out []string
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func modelsEndpoint(apiBaseURL string) (string, error) {
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
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
