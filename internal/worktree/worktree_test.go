package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "api")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "init")
	return dir
}

func TestAddAndRemoveWithRealGit(t *testing.T) {
	r := repo(t)
	c := config.Config{Layout: "sibling"}
	path, err := worktree.Add(r, "AG-1", "main", c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := worktree.Remove(r, "AG-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists: %v", err)
	}
}

func TestRemoveNeverDeletesPrimaryCheckout(t *testing.T) {
	r := repo(t)
	if err := worktree.Remove(r, "main"); err == nil {
		t.Fatal("expected primary checkout protection")
	}
}

func TestPathLayouts(t *testing.T) {
	r := "/tmp/projects/api"
	if got := worktree.Path(r, "AG-1", config.Config{Layout: "inside"}); got != "/tmp/projects/api/.worktrees/AG-1" {
		t.Fatal(got)
	}
	if got := worktree.Path(r, "AG-1", config.Config{Layout: "grouped"}); got != "/tmp/projects/api.worktrees/api.AG-1" {
		t.Fatal(got)
	}
}

func TestCurrentRepoFromLinkedWorktree(t *testing.T) {
	r := repo(t)
	path, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := worktree.CurrentRepo(path)
	got, _ = filepath.EvalSymlinks(got)
	r, _ = filepath.EvalSymlinks(r)
	if err != nil || got != r {
		t.Fatalf("got %q, %v; want %q", got, err, r)
	}
}
