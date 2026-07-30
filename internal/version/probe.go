package version

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

const probeTimeout = 5 * time.Second

type commandRunner interface {
	run(ctx context.Context, name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}

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
	Runner      commandRunner
	OSRelease   string // path to os-release; empty uses /etc/os-release
}

// Collect gathers About Page version fields.
func Collect(opts CollectOpts) AboutInfo {
	runner := opts.Runner
	if runner == nil {
		runner = execRunner{}
	}
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
		DockerImageTag: embeddedOr(unavail, ImageTag),
		BaseImageFrom:  embeddedOr(unavail, BaseImage),
	}
	info.BaseImageOS = readOSRelease(opts.OSRelease, unavail)
	info.GitCLI = probeGitCLI(ctx, runner, gitBin, unavail)
	info.OCRCLI = probeOCRCLI(ctx, runner, ocrBin, unavail)
	return info
}

func probeGitCLI(ctx context.Context, runner commandRunner, gitBin, unavailable string) string {
	out, err := runner.run(ctx, gitBin, "--version")
	if err != nil {
		return unavailable
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return unavailable
	}
	return out
}

func probeOCRCLI(ctx context.Context, runner commandRunner, ocrBin, unavailable string) string {
	out, err := runner.run(ctx, ocrBin, "version")
	if err != nil {
		return unavailable
	}
	ver, commit := parseOCRVersionOutput(out)
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
			commit = shortCommit(strings.TrimSpace(line[strings.IndexByte(line, ':')+1:]))
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
