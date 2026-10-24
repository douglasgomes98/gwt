package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "gwt-config-home")
	if err != nil {
		panic(err)
	}
	for key, value := range map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, "xdg")} {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func TestLoadDefaultsAndOptionalCommands(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil || got != (Config{Layout: "sibling", BaseBranch: "main", Editor: "code", Agent: "claude"}) {
		t.Fatalf("defaults: %+v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("editor: ''\nagent: ''\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = Load(dir)
	if err != nil || got.Editor != "" || got.Agent != "" {
		t.Fatalf("optional commands: %+v %v", got, err)
	}
}

func TestLoadAppliesAllConfiguredFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("layout: inside\nbaseBranch: feature/test\neditor: vim\nagent: codex\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	want := Config{Layout: "inside", BaseBranch: "feature/test", Editor: "vim", Agent: "codex"}
	if err != nil || got != want {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestLoadDefaultsForEmptyLocalConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	want := Config{Layout: "sibling", BaseBranch: "main", Editor: "code", Agent: "claude"}
	if err != nil || got != want {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	for _, text := range []string{
		"layout: unknown\n", "layout: ''\n", "baseBranch: ''\n",
		"unknown: value\n", "layout: [inside]\n", "layout: [\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(text), 0600); err != nil {
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
	if err := os.WriteFile(path, []byte("layout: unknown\n"), 0600); err != nil {
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
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(text), 0600); err != nil {
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
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(field+": null\n"), 0600); err != nil {
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
	if err := os.WriteFile(path, []byte("null\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error %v does not include %q", err, path)
	}
}

func TestLoadFallsBackToUserConfigLocations(t *testing.T) {
	for name, rel := range map[string]string{
		"xdg":  filepath.Join("xdg", "gwt", "config.yml"),
		"dot":  filepath.Join(".config", "gwt", "config.yml"),
		"home": "gwt.yml",
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
			path := filepath.Join(home, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("layout: branch\n"), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := Load(t.TempDir())
			if err != nil || got.Layout != "branch" {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestLoadPrefersLocalOverUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	if err := os.WriteFile(filepath.Join(home, "gwt.yml"), []byte("layout: branch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("layout: inside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got.Layout != "inside" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func writeConfig(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFindsConfigInParentDirectory(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "gwt.yml"), "layout: inside\n")
	start := filepath.Join(dir, "repo", "nested")
	if err := os.MkdirAll(start, 0700); err != nil {
		t.Fatal(err)
	}
	got, err := Load(start)
	if err != nil || got.Layout != "inside" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestLoadPrefersNearestParentConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "gwt.yml"), "layout: inside\n")
	writeConfig(t, filepath.Join(dir, "group", "gwt.yml"), "layout: grouped\n")
	got, err := Load(filepath.Join(dir, "group", "repo"))
	if err != nil || got.Layout != "grouped" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestLoadPrefersParentConfigOverUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	writeConfig(t, filepath.Join(home, ".config", "gwt", "config.yml"), "layout: branch\n")
	writeConfig(t, filepath.Join(home, "dev", "group", "gwt.yml"), "layout: inside\n")
	got, err := Load(filepath.Join(home, "dev", "group", "repo"))
	if err != nil || got.Layout != "inside" {
		t.Fatalf("got %+v, %v", got, err)
	}
	got, err = Load(filepath.Join(home, "dev", "other", "repo"))
	if err != nil || got.Layout != "branch" {
		t.Fatalf("outside group: got %+v, %v", got, err)
	}
}

func TestLoadKeepsHomeConfigBelowUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	writeConfig(t, filepath.Join(home, "gwt.yml"), "layout: inside\n")
	writeConfig(t, filepath.Join(home, ".config", "gwt", "config.yml"), "layout: branch\n")
	got, err := Load(filepath.Join(home, "dev", "repo"))
	if err != nil || got.Layout != "branch" {
		t.Fatalf("below home: got %+v, %v", got, err)
	}
	got, err = Load(home)
	if err != nil || got.Layout != "inside" {
		t.Fatalf("at home: got %+v, %v", got, err)
	}
}

func TestLoadParentValidationErrorIncludesConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gwt.yml")
	writeConfig(t, path, "layout: unknown\n")
	_, err := Load(filepath.Join(dir, "repo"))
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error %v does not include %q", err, path)
	}
}
