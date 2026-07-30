package version

import (
	"os"
	"os/exec"
	"strings"
)

type commandRunner interface {
	run(name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
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

	info := AboutInfo{
		ReviewManager:  ReviewManager(),
		DockerImageTag: embeddedOr(unavail, ImageTag),
		BaseImageFrom:  embeddedOr(unavail, BaseImage),
	}
	info.BaseImageOS = readOSRelease(opts.OSRelease, unavail)
	info.GitCLI = probeGitCLI(runner, gitBin, unavail)
	info.OCRCLI = probeOCRCLI(runner, ocrBin, unavail)
	return info
}

func probeGitCLI(runner commandRunner, gitBin, unavailable string) string {
	out, err := runner.run(gitBin, "--version")
	if err != nil {
		return unavailable
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return unavailable
	}
	return out
}

func probeOCRCLI(runner commandRunner, ocrBin, unavailable string) string {
	out, err := runner.run(ocrBin, "version")
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
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "version:") {
			version = strings.TrimSpace(line[strings.IndexByte(line, ':')+1:])
			continue
		}
		if strings.HasPrefix(lower, "commit:") {
			commit = shortCommit(strings.TrimSpace(line[strings.IndexByte(line, ':')+1:]))
			continue
		}
		if strings.HasPrefix(lower, "open-code-review version") {
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
