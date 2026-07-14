package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadValidationErrorIncludesConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gwt.yml")
	if err := os.WriteFile(path, []byte("layout: unknown\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error %v does not include %q", err, path)
	}
}

func TestLoadRejectsSecondDocument(t *testing.T) {
	for _, text := range []string{
		"layout: sibling\n---\nlayout: inside\n",
		"layout: sibling\n---\nunknown: value\n",
		"layout: sibling\n---\nlayout: [inside]\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatalf("Load accepted multiple documents: %q", text)
		}
	}
}

func TestLoadRejectsNullFields(t *testing.T) {
	for _, field := range []string{"layout", "baseBranch", "editor", "agent"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(field+": null\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatalf("Load accepted null %s", field)
		}
	}
}

func TestLoadRejectsTopLevelNullConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gwt.yml")
	if err := os.WriteFile(path, []byte("null\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error %v does not include %q", err, path)
	}
}
