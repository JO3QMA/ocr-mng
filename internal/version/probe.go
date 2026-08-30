package version

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

const probeTimeout = 5 * time.Second

// AboutInfo is the data shown on the About Page.
type AboutInfo struct {
	ReviewManager  string
	DockerImageTag string
	BaseImageFrom  string
	BaseImageOS    string
	GitCLI         string
	OCRCLI         string
}

// CollectOpts configures runtime version collection.
type CollectOpts struct {
	Context     context.Context
	Unavailable string
	GitBinary   string
	OCRBinary   string
	OSRelease   string // path to os-release; empty uses /etc/os-release
}

// Collect gathers About Page version fields.
func Collect(opts CollectOpts) AboutInfo {
	gitBin := strings.TrimSpace(opts.GitBinary)
	if gitBin == "" {
		gitBin = "git"
	}
	ocrBin := strings.TrimSpace(opts.OCRBinary)
	if ocrBin == "" {
		ocrBin = "ocr"
	}
	unavail := opts.Unavailable

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	info := AboutInfo{
		ReviewManager:  ReviewManager(),
		DockerImageTag: embeddedOr(ImageTag, unavail),
		BaseImageFrom:  embeddedOr(BaseImage, unavail),
	}
	info.BaseImageOS = readOSRelease(opts.OSRelease, unavail)
	info.GitCLI = probeGitCLI(ctx, gitBin, unavail)
	info.OCRCLI = probeOCRCLI(ctx, ocrBin, unavail)
	return info
}

func probeGitCLI(ctx context.Context, gitBin, unavailable string) string {
	cmd := exec.CommandContext(ctx, gitBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		return unavailable
	}
	outStr := strings.TrimSpace(string(out))
	if outStr == "" {
		return unavailable
	}
	return outStr
}

func probeOCRCLI(ctx context.Context, ocrBin, unavailable string) string {
	cmd := exec.CommandContext(ctx, ocrBin, "version")
	out, err := cmd.Output()
	if err != nil {
		return unavailable
	}
	ver, commit := parseOCRVersionOutput(string(out))
	if ver == "" {
		return unavailable
	}
	if commit == "" {
		return ver
	}
	return ver + " (" + commit + ")"
}

func parseOCRVersionOutput(out string) (version, commit string) {
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if version != "" && commit != "" {
			break
		}
		lower := strings.ToLower(line)
		if version == "" && strings.HasPrefix(lower, "version:") {
			version = strings.TrimSpace(line[strings.IndexByte(line, ':')+1:])
			continue
		}
		if commit == "" && strings.HasPrefix(lower, "commit:") {
			commit = ShortCommit(strings.TrimSpace(line[strings.IndexByte(line, ':')+1:]))
			continue
		}
		if version == "" && strings.HasPrefix(lower, "open-code-review version") {
			version = strings.TrimSpace(line[len("open-code-review version"):])
		}
	}
	return version, commit
}

func readOSRelease(path, unavailable string) string {
	if path == "" {
		path = "/etc/os-release"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return unavailable
	}
	name := parseOSReleaseContent(string(b))
	if name == "" {
		return unavailable
	}
	return name
}

func parseOSReleaseContent(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		v := strings.TrimPrefix(line, "PRETTY_NAME=")
		return strings.Trim(v, `"`)
	}
	return ""
}
