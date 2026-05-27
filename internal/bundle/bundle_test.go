package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackExtractRoundTrip(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "manifest.json"), []byte(`{"name":"x"}`))
	mustWrite(t, filepath.Join(src, "wrapper"), []byte("#!/bin/sh\necho hi\n"))
	mustWrite(t, filepath.Join(src, "engine", "main.go"), []byte("package main\n"))

	data, hash, err := Pack(src)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if hash != Hash(data) {
		t.Fatalf("hash mismatch: got %s, want %s", hash, Hash(data))
	}

	dest := filepath.Join(t.TempDir(), "out")
	if err := Extract(data, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}

	for _, rel := range []string{"manifest.json", "wrapper", "engine/main.go"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("missing extracted file %s: %v", rel, err)
		}
	}

	// Second extract should be a no-op (dest exists).
	if err := Extract(data, dest); err != nil {
		t.Errorf("second extract should be a no-op, got %v", err)
	}
}

func TestPackDeterministic(t *testing.T) {
	build := func(t *testing.T) string {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "a"), []byte("alpha"))
		mustWrite(t, filepath.Join(dir, "nested", "b"), []byte("beta"))
		return dir
	}

	_, h1, err := Pack(build(t))
	if err != nil {
		t.Fatalf("pack 1: %v", err)
	}
	_, h2, err := Pack(build(t))
	if err != nil {
		t.Fatalf("pack 2: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hashes should match across runs: %s vs %s", h1, h2)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
