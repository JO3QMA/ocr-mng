package review_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/config"
	"github.com/jo3qma/ocr-mng/internal/review"
	"github.com/jo3qma/ocr-mng/internal/store"
)

func TestNewEngine(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/rm.db", []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	e := review.NewEngine(config.Config{}, st, slog.Default())
	if e == nil {
		t.Fatal("nil engine")
	}
	gs, err := st.GetGlobalSettings(context.Background())
	if err != nil || gs.MaxConcurrentReviews < 1 {
		t.Fatalf("settings: %+v err=%v", gs, err)
	}
}
