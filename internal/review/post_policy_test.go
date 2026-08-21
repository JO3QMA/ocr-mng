package review

import (
	"errors"
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestShouldPostOCRResult(t *testing.T) {
	ocrErr := errors.New("review failed: all 6 file review(s) failed")
	failed := ocr.Result{
		Status:  "failed",
		Message: "Review failed: 0 finding(s); 6 of 6 selected item(s) failed.",
	}
	cases := []struct {
		name   string
		result ocr.Result
		ocrErr error
		want   bool
	}{
		{name: "clean zero finding", result: ocr.Result{Message: "ok"}, want: true},
		{name: "failed no comments", result: failed, ocrErr: ocrErr, want: false},
		{name: "failed exit 0", result: failed, want: false},
		{name: "cli error empty json", result: ocr.Result{Comments: []ocr.Comment{}}, ocrErr: ocrErr, want: false},
		{name: "partial warnings", result: ocr.Result{Warnings: ocr.Warnings{{Message: "boom"}}}, ocrErr: ocrErr, want: true},
		{name: "completed_with_errors", result: ocr.Result{Status: "completed_with_errors"}, want: true},
		{name: "failed with comments", result: ocr.Result{Status: "failed", Comments: []ocr.Comment{{Content: "x"}}}, want: true},
		{name: "cli error with comments", result: ocr.Result{Comments: []ocr.Comment{{Content: "x"}}}, ocrErr: ocrErr, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldPostOCRResult(tc.result, tc.ocrErr); got != tc.want {
				t.Fatalf("shouldPostOCRResult() = %v, want %v", got, tc.want)
			}
		})
	}
}
