package org

import (
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key := generateAPIKey()
	if len(key) < 10 {
		t.Errorf("key too short: %s", key)
	}
	if key[:4] != "nxk_" {
		t.Errorf("expected nxk_ prefix, got %s", key[:4])
	}

	key2 := generateAPIKey()
	if key == key2 {
		t.Error("expected unique keys")
	}
}
