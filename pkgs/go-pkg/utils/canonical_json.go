package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func CanonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func FingerprintSHA256(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
