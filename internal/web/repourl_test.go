package web

import "testing"

func TestParseRepoURL(t *testing.T) {
	const gh = "https://github.com"
	tests := []struct {
		name    string
		raw     string
		base    string
		owner   string
		repo    string
		errKey  string
	}{
		{
			name:  "web url",
			raw:   "https://github.com/JO3QMA/ocr-mng",
			base:  gh,
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "git suffix",
			raw:   "https://github.com/JO3QMA/ocr-mng.git",
			base:  gh,
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "trailing slash on url",
			raw:   "https://github.com/JO3QMA/ocr-mng/",
			base:  gh,
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "trailing slash on base",
			raw:   "https://github.com/JO3QMA/ocr-mng",
			base:  "https://github.com/",
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "host case",
			raw:   "https://GitHub.com/JO3QMA/ocr-mng",
			base:  "https://github.com",
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "query ignored",
			raw:   "https://github.com/JO3QMA/ocr-mng?tab=readme-ov-file",
			base:  gh,
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "fragment ignored",
			raw:   "https://github.com/JO3QMA/ocr-mng#readme",
			base:  gh,
			owner: "JO3QMA", repo: "ocr-mng",
		},
		{
			name:  "gitea subpath",
			raw:   "https://git.example.com/gitea/org/repo",
			base:  "https://git.example.com/gitea",
			owner: "org", repo: "repo",
		},
		{
			name:   "http rejected",
			raw:    "http://github.com/JO3QMA/ocr-mng",
			base:   gh,
			errKey: "form.repo_url_https",
		},
		{
			name:   "userinfo rejected",
			raw:    "https://user:token@github.com/JO3QMA/ocr-mng",
			base:   gh,
			errKey: "form.repo_url_userinfo",
		},
		{
			name:   "host mismatch",
			raw:    "https://github.com/JO3QMA/ocr-mng",
			base:   "https://git.example.com",
			errKey: "form.repo_url_host_mismatch",
		},
		{
			name:   "extra path",
			raw:    "https://github.com/JO3QMA/ocr-mng/tree/main",
			base:   gh,
			errKey: "form.repo_url_path",
		},
		{
			name:   "missing name",
			raw:    "https://github.com/JO3QMA",
			base:   gh,
			errKey: "form.repo_url_path",
		},
		{
			name:   "scheme-less",
			raw:    "github.com/JO3QMA/ocr-mng",
			base:   gh,
			errKey: "form.repo_url_invalid",
		},
		{
			name:   "empty",
			raw:    "  ",
			base:   gh,
			errKey: "form.repo_url_required",
		},
		{
			name:   "gitea prefix miss",
			raw:    "https://git.example.com/other/org/repo",
			base:   "https://git.example.com/gitea",
			errKey: "form.repo_url_host_mismatch",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, name, err := ParseRepoURL(tc.raw, tc.base)
			if tc.errKey != "" {
				if err == nil || err.Error() != tc.errKey {
					t.Fatalf("err=%v want %s", err, tc.errKey)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if owner != tc.owner || name != tc.repo {
				t.Fatalf("got %s/%s want %s/%s", owner, name, tc.owner, tc.repo)
			}
		})
	}
}

func TestFormatRepoURL(t *testing.T) {
	got := FormatRepoURL("https://github.com/", "JO3QMA", "ocr-mng")
	want := "https://github.com/JO3QMA/ocr-mng"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = FormatRepoURL("https://git.example.com/gitea", "org", "repo")
	want = "https://git.example.com/gitea/org/repo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
