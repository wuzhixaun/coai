package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeStorageFile(t *testing.T, dir, name string, size int, modAgo time.Duration, now time.Time) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, make([]byte, size), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	mt := now.Add(-modAgo)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
	return p
}

func TestEnforceStorageLimits_TTL(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := writeStorageFile(t, dir, "old.png", 10, 48*time.Hour, now)
	fresh := writeStorageFile(t, dir, "fresh.png", 10, 1*time.Hour, now)

	removed, _ := enforceStorageLimits([]string{dir}, 24*time.Hour, 0, now)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old file should be deleted")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh file should be kept: %v", err)
	}
}

func TestEnforceStorageLimits_SizeCap(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	// three 100-byte files, total 300; cap at 150 -> must delete the two oldest.
	writeStorageFile(t, dir, "a.png", 100, 3*time.Hour, now)
	writeStorageFile(t, dir, "b.png", 100, 2*time.Hour, now)
	newest := writeStorageFile(t, dir, "c.png", 100, 1*time.Hour, now)

	removed, freed := enforceStorageLimits([]string{dir}, 0, 150, now)
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}
	if freed != 200 {
		t.Fatalf("expected 200 bytes freed, got %d", freed)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Fatalf("newest file should be kept: %v", err)
	}
}

func TestEnforceStorageLimits_Disabled(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStorageFile(t, dir, "x.png", 10, 100*time.Hour, now)

	removed, _ := enforceStorageLimits([]string{dir}, 0, 0, now)
	if removed != 0 {
		t.Fatalf("expected nothing removed when disabled, got %d", removed)
	}
}
