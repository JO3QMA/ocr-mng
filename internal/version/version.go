package version

import "strings"

// Set at link time via -ldflags -X.
var (
	Version   = "dev"
	Commit    = ""
	ImageTag  = ""
	BaseImage = ""
)

// ReviewManager returns the Review Manager Version display string.
func ReviewManager() string {
	ver := strings.TrimSpace(Version)
	if ver == "" {
		ver = "dev"
	}
	c := ShortCommit(Commit)
	if c == "" {
		return ver
	}
	return ver + " (" + c + ")"
}

// ShortCommit returns a trimmed commit hash for display (max 7 chars).
func ShortCommit(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func embeddedOr(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}
