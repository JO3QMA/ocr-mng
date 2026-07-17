package ocr_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/ocr"
)

func TestInferProtocol(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"", ""},
		{"https://api.openai.com/v1", ocr.ProtocolOpenAI},
		{"https://api.openai.com/v1/responses", ocr.ProtocolOpenAIResponses},
		{"https://gateway.example/v1/responses/", ocr.ProtocolOpenAIResponses},
		{"https://api.anthropic.com", ocr.ProtocolAnthropic},
		{"https://my-anthropic-proxy.internal/v1", ocr.ProtocolAnthropic},
		{"https://llm.internal/v1", ocr.ProtocolOpenAI},
	}
	for _, tc := range cases {
		if got := ocr.InferProtocol(tc.url); got != tc.want {
			t.Errorf("InferProtocol(%q)=%q want %q", tc.url, got, tc.want)
		}
	}
}
