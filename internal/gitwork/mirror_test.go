package gitwork_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/gitwork"
)

func TestFromRef(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "origin/main"},
		{"main", "origin/main"},
		{"origin/develop", "origin/develop"},
		{"  release ", "origin/release"},
	}
	for _, tc := range tests {
		if got := gitwork.FromRef(tc.in); got != tc.want {
			t.Fatalf("FromRef(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
