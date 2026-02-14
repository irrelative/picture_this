package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMigrationVersions(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"20251220191202_init.up.sql",
		"20251220191202_init.down.sql",
		"20260104120000_prompt_joke_audio_path.up.sql",
		"20260213160000_prompt_library_embeddings.up.sql",
		"README.md",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	versions, err := migrationVersions(dir)
	if err != nil {
		t.Fatalf("migrationVersions returned error: %v", err)
	}

	expected := []uint{20251220191202, 20260104120000, 20260213160000}
	if !reflect.DeepEqual(versions, expected) {
		t.Fatalf("unexpected versions; got=%v want=%v", versions, expected)
	}
}

func TestDetermineForceTarget(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"20251220191202_init.up.sql",
		"20260104120000_prompt_joke_audio_path.up.sql",
		"20260213160000_prompt_library_embeddings.up.sql",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	target, source, err := determineForceTarget(-1, 20260213160000, dir)
	if err != nil {
		t.Fatalf("determineForceTarget returned error: %v", err)
	}
	if target != 20260104120000 {
		t.Fatalf("unexpected target; got=%d want=%d", target, 20260104120000)
	}
	if source != "previous migration file" {
		t.Fatalf("unexpected source; got=%q", source)
	}

	explicitTarget, explicitSource, err := determineForceTarget(123, 20260213160000, dir)
	if err != nil {
		t.Fatalf("determineForceTarget explicit returned error: %v", err)
	}
	if explicitTarget != 123 || explicitSource != "explicit --to" {
		t.Fatalf("unexpected explicit target/source; got=%d/%q", explicitTarget, explicitSource)
	}
}
