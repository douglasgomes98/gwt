package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/douglasgomes/gwt/internal/config"
)

func testRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	initRepo(t, dir)
	return dir
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
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
	if !bytes.Contains(out.Bytes(), []byte(path+"\tAG-1\t(clean)\n")) {
		t.Fatalf("list output: %q", out.String())
	}
	if e := a.Run([]string{"prune"}); e != nil {
		t.Fatal(e)
	}
	if e := a.Run([]string{"rm", "AG-1"}); e != nil {
		t.Fatal(e)
	}
}

func TestAddFlagsCanAppearEitherSideButCannotMix(t *testing.T) {
	for _, args := range [][]string{{"add", "AG-1", "-e"}, {"add", "-e", "AG-2"}} {
		a := New(&bytes.Buffer{}, &bytes.Buffer{}, testRepo(t), "", config.Config{Layout: "sibling", BaseBranch: "main", Editor: "true"})
		if err := a.Run(args); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "AG-3", "-e", "-a"}, {"add", "AG-3", "--all", "-e"}} {
		a := New(&bytes.Buffer{}, &bytes.Buffer{}, testRepo(t), "", config.Config{Layout: "sibling", BaseBranch: "main"})
		if err := a.Run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestRemoveAllPrevalidatesPrimary(t *testing.T) {
	dir := testRepo(t)
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "sibling", BaseBranch: "main"})
	if err := a.Run([]string{"add", "AG-1"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Run([]string{"rm", "main", "--all"}); err == nil {
		t.Fatal("rm --all accepted the primary checkout")
	}
	if err := a.Run([]string{"open", "AG-1", "-p"}); err == nil {
		t.Fatal("open accepted removed -p flag")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".AG-1")); err != nil {
		t.Fatalf("worktree removed before validation: %v", err)
	}
}

func TestPruneRejectsArguments(t *testing.T) {
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, testRepo(t), "", config.Config{})
	if err := a.Run([]string{"prune", "extra"}); err == nil {
		t.Fatal("prune accepted arguments")
	}
}

func TestAddAllDoesNotRequireRemote(t *testing.T) {
	dir := testRepo(t)
	sibling := filepath.Join(filepath.Dir(dir), "sibling")
	initRepo(t, sibling)
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "sibling", BaseBranch: "main"})
	if err := a.Run([]string{"add", "--all", "AG-1"}); err != nil {
		t.Fatal(err)
	}
	for _, repo := range []string{dir, sibling} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".AG-1")); err != nil {
			t.Fatalf("worktree for %s was not created: %v", repo, err)
		}
	}
}

func TestRemoveAllIgnoresSiblingWithoutBranch(t *testing.T) {
	dir := testRepo(t)
	initRepo(t, filepath.Join(filepath.Dir(dir), "sibling"))
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "sibling", BaseBranch: "main"})
	if err := a.Run([]string{"add", "AG-1"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Run([]string{"rm", "--all", "AG-1"}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenUsesShellForExistingWorktree(t *testing.T) {
	dir := testRepo(t)
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "sibling", BaseBranch: "main"})
	if err := a.Run([]string{"add", "AG-1"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "true")
	if err := a.Run([]string{"open", "AG-1"}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateFetchesCleanBaseRoot(t *testing.T) {
	dir := testRepo(t)
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{BaseBranch: "main"})
	err := a.Run([]string{"update"})
	if err == nil || !strings.Contains(err.Error(), "git fetch origin main") {
		t.Fatalf("update error = %v, want fetch failure", err)
	}
}

func TestUpdateRejectsDirtyRootBeforeFetch(t *testing.T) {
	dir := testRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "dirty"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{BaseBranch: "main"})
	err := a.Run([]string{"update"})
	if err == nil || !strings.Contains(err.Error(), "root has uncommitted changes") {
		t.Fatalf("update error = %v, want dirty-root failure", err)
	}
}

func TestUpdateRejectsWrongBranchBeforeFetch(t *testing.T) {
	dir := testRepo(t)
	cmd := exec.Command("git", "checkout", "-b", "other")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, out)
	}
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{BaseBranch: "main"})
	err := a.Run([]string{"update"})
	if err == nil || !strings.Contains(err.Error(), "root must be on main") {
		t.Fatalf("update error = %v, want wrong-branch failure", err)
	}
}

func TestCommandErrorsAndHelpers(t *testing.T) {
	a := App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Dir: t.TempDir()}
	for _, args := range [][]string{nil, {"unknown"}, {"--version"}, {"-version"}, {"help", "add"}, {"add"}, {"open"}, {"rm"}, {"list", "extra"}} {
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
