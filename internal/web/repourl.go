package web

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ParseRepoURL extracts Owner/Name from a Repo URL under webBaseURL.
// err.Error() is an i18n key (form.repo_url_*) on failure.
func ParseRepoURL(raw, webBaseURL string) (owner, name string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("form.repo_url_required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("form.repo_url_invalid")
	}
	if strings.ToLower(u.Scheme) != "https" {
		return "", "", fmt.Errorf("form.repo_url_https")
	}
	if u.User != nil {
		return "", "", fmt.Errorf("form.repo_url_userinfo")
	}

	base, err := parseWebBase(webBaseURL)
	if err != nil {
		return "", "", fmt.Errorf("form.repo_url_bad_base")
	}
	if strings.ToLower(u.Scheme) != base.Scheme || !sameHostPort(u, base) {
		return "", "", fmt.Errorf("form.repo_url_host_mismatch")
	}

	rest, ok := pathUnderBase(u.Path, base.Path)
	if !ok {
		return "", "", fmt.Errorf("form.repo_url_host_mismatch")
	}
	rest = strings.Trim(rest, "/")
	if len(rest) >= 4 && strings.EqualFold(rest[len(rest)-4:], ".git") {
		rest = rest[:len(rest)-4]
		rest = strings.TrimSuffix(rest, "/")
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("form.repo_url_path")
	}
	owner, err = url.PathUnescape(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("form.repo_url_path")
	}
	name, err = url.PathUnescape(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("form.repo_url_path")
	}
	owner, name = strings.TrimSpace(owner), strings.TrimSpace(name)
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("form.repo_url_path")
	}
	return owner, name, nil
}

// FormatRepoURL builds an edit-form Repo URL (no .git) from WebBaseURL and Owner/Name.
func FormatRepoURL(webBaseURL, owner, name string) string {
	owner = strings.TrimSpace(owner)
	name = strings.TrimSpace(name)
	base := strings.TrimSpace(webBaseURL)
	if owner == "" || name == "" || base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func parseWebBase(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func sameHostPort(a, b *url.URL) bool {
	ah, ap := hostnamePort(a)
	bh, bp := hostnamePort(b)
	return ah == bh && ap == bp
}

func hostnamePort(u *url.URL) (host, port string) {
	host = strings.ToLower(u.Hostname())
	port = u.Port()
	if port != "" {
		return host, port
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return host, "443"
	case "http":
		return host, "80"
	default:
		return host, ""
	}
}

// pathUnderBase reports whether uPath is under basePath and returns the remainder
// (without a leading slash), e.g. base /gitea + /gitea/org/repo → org/repo.
func pathUnderBase(uPath, basePath string) (rest string, ok bool) {
	uPath = cleanURLPath(uPath)
	basePath = cleanURLPath(basePath)
	if basePath == "" || basePath == "/" {
		return strings.TrimPrefix(uPath, "/"), true
	}
	if uPath == basePath {
		return "", true
	}
	prefix := basePath + "/"
	if !strings.HasPrefix(uPath, prefix) {
		return "", false
	}
	return uPath[len(prefix):], true
}

func cleanURLPath(p string) string {
	if p == "" {
		return ""
	}
	c := path.Clean(p)
	if c == "." {
		return ""
	}
	return c
}
