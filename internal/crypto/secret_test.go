package crypto_test

import (
	"testing"

	"github.com/jo3qma/ocr-mng/internal/crypto"
)

func TestEncryptRoundTrip(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	enc, err := crypto.Encrypt(key, []byte("pat-secret"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := crypto.Decrypt(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "pat-secret" {
		t.Fatalf("got %q", out)
	}
}
