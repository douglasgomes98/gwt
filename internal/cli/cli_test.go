package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/douglasgomes/gwt/internal/config"
)

func TestFlagsFirstKeepsAliasStyle(t *testing.T) {
	got := flagsFirst([]string{"AG-1", "main", "-e", "--all"})
	want := []string{"-e", "--all", "AG-1", "main"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func testRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestCommandsUseRealWorktree(t *testing.T) {
	dir := testRepo(t)
	var out, err bytes.Buffer
	a := App{Out: &out, Err: &err, Dir: dir, Config: config.Config{Layout: "sibling", BaseBranch: "main"}}
	if e := a.Run([]string{"add", "AG-1"}); e != nil {
		t.Fatal(e)
	}
	path := strings.TrimSpace(out.String())
	if filepath.Base(path) != "repo.AG-1" {
		t.Fatalf("add output: %q", out.String())
	}
	out.Reset()
	if e := a.Run([]string{"list"}); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte(path+"\tAG-1\n")) {
		t.Fatalf("list output: %q", out.String())
	}
	out.Reset()
	if e := a.Run([]string{"open", "AG-1", "-p"}); e != nil || out.String() != path+"\n" {
		t.Fatalf("open: %v, %q", e, out.String())
	}
	if e := a.Run([]string{"prune"}); e != nil {
		t.Fatal(e)
	}
	if e := a.Run([]string{"rm", "AG-1"}); e != nil {
		t.Fatal(e)
	}
}

func TestCommandErrorsAndHelpers(t *testing.T) {
	a := App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Dir: t.TempDir()}
	for _, args := range [][]string{nil, {"unknown"}, {"help", "add"}, {"add"}, {"open"}, {"rm"}, {"list", "extra"}} {
		if err := a.Run(args); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	if err := runAt("", t.TempDir()); err == nil {
		t.Fatal("missing command must fail")
	}
	if err := runAt("true", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "true")
	if err := shell(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestHelpListsCommands(t *testing.T) {
	var out bytes.Buffer
	if err := New(&out, &bytes.Buffer{}, t.TempDir(), "test", config.Config{}).Run([]string{"help"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("add <branch>")) {
		t.Fatalf("unexpected help: %q", got)
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	a := New(&out, &bytes.Buffer{}, t.TempDir(), "v1.2.3", config.Config{})
	if err := a.Run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "v1.2.3\n" {
		t.Fatalf("version output: %q", got)
	}
	if err := a.Run([]string{"version", "extra"}); err == nil {
		t.Fatal("version with extra arguments must fail")
	}
}
