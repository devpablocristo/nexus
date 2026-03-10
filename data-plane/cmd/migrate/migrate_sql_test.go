package main

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"

	"nexus/pkg/utils"
)

func TestLoadMigrations_SortsAndPairs(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "0002_core.up.sql"), "SELECT 2;")
	mustWrite(t, filepath.Join(dir, "0002_core.down.sql"), "SELECT 2;")
	mustWrite(t, filepath.Join(dir, "0001_ext.up.sql"), "SELECT 1;")
	mustWrite(t, filepath.Join(dir, "0001_ext.down.sql"), "SELECT 1;")

	migs, err := utils.LoadMigrations(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(migs) != 2 {
		t.Fatalf("expected 2, got %d", len(migs))
	}
	if migs[0].Version != 1 || migs[1].Version != 2 {
		t.Fatalf("expected sorted versions 1,2 got %d,%d", migs[0].Version, migs[1].Version)
	}
}

func mustWrite(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
