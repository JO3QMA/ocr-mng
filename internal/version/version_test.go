package version

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeRunner map[string]string

func (f fakeRunner) run(ctx context.Context, name string, args ...string) (string, error) {
	key := name
	if len(args) > 0 {
		key += " " + args[0]
	}
	out, ok := f[key]
	if !ok {
		return "", errors.New("not found")
	}
	return out, nil
}

func TestReviewManager(t *testing.T) {
	oldV, oldC := Version, Commit
	t.Cleanup(func() { Version, Commit = oldV, oldC })

	Version, Commit = "v1.2.3", "abc1234567"
	if got := ReviewManager(); got != "v1.2.3 (abc1234)" {
		t.Fatalf("got %q", got)
	}
	Version, Commit = "dev", "deadbeef"
	if got := ReviewManager(); got != "dev (deadbee)" {
		t.Fatalf("got %q", got)
	}
	Version, Commit = "dev", ""
	if got := ReviewManager(); got != "dev" {
		t.Fatalf("got %q", got)
	}
}

func TestParseOCRVersionOutput(t *testing.T) {
	ver, commit := parseOCRVersionOutput("version: 1.0.9\ncommit: abc1234567\n")
	if ver != "1.0.9" || commit != "abc1234" {
		t.Fatalf("got %q %q", ver, commit)
	}
	ver, commit = parseOCRVersionOutput("open-code-review version 2.0.0\n")
	if ver != "2.0.0" || commit != "" {
		t.Fatalf("got %q %q", ver, commit)
	}
	ver, commit = parseOCRVersionOutput("version: 1.0.0\ncommit: aaa1111\nversion: 9.9.9\ncommit: bbb2222\n")
	if ver != "1.0.0" || commit != "aaa1111" {
		t.Fatalf("first match: got %q %q", ver, commit)
	}
}

func TestParseOSReleaseContent(t *testing.T) {
	const sample = `NAME="Debian GNU/Linux"
PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
VERSION_ID="12"
`
	if got := parseOSReleaseContent(sample); got != "Debian GNU/Linux 12 (bookworm)" {
		t.Fatalf("got %q", got)
	}
}

func TestCollect(t *testing.T) {
	oldTag, oldBase := ImageTag, BaseImage
	t.Cleanup(func() { ImageTag, BaseImage = oldTag, oldBase })

	ImageTag = "sha-abc1234"
	BaseImage = "debian:bookworm-slim"

	dir := t.TempDir()
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte(`PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"`), 0o644); err != nil {
		t.Fatal(err)
	}

	info := Collect(CollectOpts{
		Unavailable: "N/A",
		GitBinary:   "git",
		OCRBinary:   "ocr",
		OSRelease:   osRelease,
		Runner: fakeRunner{
			"git --version": "git version 2.43.0\n",
			"ocr version":   "version: 1.0.9\ncommit: feedface\n",
		},
	})
	if info.DockerImageTag != "sha-abc1234" {
		t.Fatalf("docker tag: %q", info.DockerImageTag)
	}
	if info.BaseImageFrom != "debian:bookworm-slim" {
		t.Fatalf("base from: %q", info.BaseImageFrom)
	}
	if info.BaseImageOS != "Debian GNU/Linux 12 (bookworm)" {
		t.Fatalf("base os: %q", info.BaseImageOS)
	}
	if info.GitCLI != "git version 2.43.0" {
		t.Fatalf("git: %q", info.GitCLI)
	}
	if info.OCRCLI != "1.0.9 (feedfac)" {
		t.Fatalf("ocr: %q", info.OCRCLI)
	}
}

func TestCollectUnavailable(t *testing.T) {
	oldTag, oldBase := ImageTag, BaseImage
	t.Cleanup(func() { ImageTag, BaseImage = oldTag, oldBase })
	ImageTag, BaseImage = "", ""

	info := Collect(CollectOpts{
		Unavailable: "N/A",
		OSRelease:   filepath.Join(t.TempDir(), "missing"),
		Runner:      fakeRunner{},
	})
	if info.DockerImageTag != "N/A" || info.BaseImageFrom != "N/A" {
		t.Fatalf("embedded: docker=%q base=%q", info.DockerImageTag, info.BaseImageFrom)
	}
	if info.BaseImageOS != "N/A" || info.GitCLI != "N/A" || info.OCRCLI != "N/A" {
		t.Fatalf("probes: os=%q git=%q ocr=%q", info.BaseImageOS, info.GitCLI, info.OCRCLI)
	}
}
