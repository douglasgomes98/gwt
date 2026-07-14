package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndOptionalCommands(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil || got != (Config{Layout: "sibling", BaseBranch: "main", Editor: "code", Agent: "claude"}) {
		t.Fatalf("defaults: %+v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("editor: ''\nagent: ''\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = Load(dir)
	if err != nil || got.Editor != "" || got.Agent != "" {
		t.Fatalf("optional commands: %+v %v", got, err)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	for _, text := range []string{
		"layout: unknown\n", "layout: ''\n", "baseBranch: ''\n",
		"unknown: value\n", "layout: [inside]\n", "layout: [\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatalf("Load accepted %q", text)
		}
	}
}
