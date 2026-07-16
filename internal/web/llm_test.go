package web

import "testing"

func TestParseLLMPairField(t *testing.T) {
	pid, mid, err := parseLLMPairField("0:0")
	if err != nil || pid != 0 || mid != 0 {
		t.Fatalf("empty: %d %d %v", pid, mid, err)
	}
	pid, mid, err = parseLLMPairField("3:9")
	if err != nil || pid != 3 || mid != 9 {
		t.Fatalf("pair: %d %d %v", pid, mid, err)
	}
	if _, _, err := parseLLMPairField("3"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := parseLLMPairField("1:0"); err == nil {
		t.Fatal("expected partial reject")
	}
}
