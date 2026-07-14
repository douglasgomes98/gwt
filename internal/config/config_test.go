package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndLocalConfig(t *testing.T) {
	dir := t.TempDir()
	if got := Load(dir); got.Layout != "sibling" || got.BaseBranch != "main" || got.Editor != "code" {
		t.Fatalf("defaults: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("layout: inside\nbaseBranch: trunk\neditor: vim\nagent: codex\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := Load(dir)
	if got != (Config{Layout: "inside", BaseBranch: "trunk", Editor: "vim", Agent: "codex"}) {
		t.Fatalf("config: %+v", got)
	}
}

func TestLoadFallsBackForEmptyValues(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("layout: ''\nbaseBranch: ''\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := Load(dir)
	if got.Layout != "sibling" || got.BaseBranch != "main" {
		t.Fatalf("config: %+v", got)
	}
}
