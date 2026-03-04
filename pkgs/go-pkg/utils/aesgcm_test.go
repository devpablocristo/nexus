package utils

import (
	"encoding/base64"
	"testing"
)

func TestAESGCMRoundtrip(t *testing.T) {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	enc, err := NewAESGCM(base64.StdEncoding.EncodeToString(k))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ct, nonce, err := enc.Encrypt([]byte("secret-value"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pt, err := enc.Decrypt(ct, nonce)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(pt) != "secret-value" {
		t.Fatalf("unexpected plaintext: %s", string(pt))
	}
}
