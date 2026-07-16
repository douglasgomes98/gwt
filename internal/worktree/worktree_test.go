package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/douglasgomes98/gwt/internal/config"
	"github.com/douglasgomes98/gwt/internal/worktree"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) // #nosec G204 -- test invokes Git with fixed arguments.
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "api")
	if err := os.Mkdir(dir, 0750); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "init")
	return dir
}

func remoteRepos(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote.git")
	git(t, dir, "init", "--bare", "--initial-branch=main", remote)
	root := filepath.Join(dir, "root")
	git(t, dir, "clone", remote, root)
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "init")
	git(t, root, "push", "-u", "origin", "main")
	peer := filepath.Join(dir, "peer")
	git(t, dir, "clone", remote, peer)
	git(t, peer, "config", "user.email", "test@example.com")
	git(t, peer, "config", "user.name", "Test")
	return root, peer
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

func TestRemoveRejectsDetachedWorktree(t *testing.T) {
	r := repo(t)
	path, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	git(t, path, "checkout", "--detach")
	if err := worktree.Remove(r, ""); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveAllRemovesOnlyNonPrimaryWorktrees(t *testing.T) {
	r := repo(t)
	for _, branch := range []string{"AG-1", "AG-2"} {
		if _, err := worktree.Add(r, branch, "main", config.Config{Layout: "sibling"}); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := worktree.RemoveAll(r)
	if err != nil || removed != 2 {
		t.Fatalf("RemoveAll: %d, %v", removed, err)
	}
	items, err := worktree.ListFast(r)
	if err != nil || len(items) != 1 || !items[0].Primary {
		t.Fatalf("items: %#v, %v", items, err)
	}
}

func TestRemoveAllRejectsDetachedWorktreeBeforeRemovingOthers(t *testing.T) {
	r := repo(t)
	first, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := worktree.Add(r, "AG-2", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	git(t, second, "checkout", "--detach")
	if _, err := worktree.RemoveAll(r); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("RemoveAll error: %v", err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("non-detached worktree removed: %v", err)
	}
}

func TestRemoveAllRejectsNonRepository(t *testing.T) {
	if _, err := worktree.RemoveAll(t.TempDir()); err == nil {
		t.Fatal("RemoveAll accepted a non-repository")
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
	if got := worktree.Path(r, "feat/add-user", config.Config{Layout: "inside"}); got != "/tmp/projects/api/.worktrees/feat-add-user" {
		t.Fatal(got)
	}
	if got := worktree.Path(r, "feat/add-user", config.Config{Layout: "grouped"}); got != "/tmp/projects/api.worktrees/api.feat-add-user" {
		t.Fatal(got)
	}
	if got := worktree.Path(r, "feat/add-user", config.Config{Layout: "sibling"}); got != "/tmp/projects/api.feat-add-user" {
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

func TestListAndRepositoryHelpers(t *testing.T) {
	r := repo(t)
	wantRoot, _ := filepath.EvalSymlinks(filepath.Dir(r))
	if got := worktree.Root(r); got != wantRoot {
		t.Fatalf("root: %q", got)
	}
	repos, err := worktree.Repos(r)
	resolvedRepo, _ := filepath.EvalSymlinks(r)
	if err != nil || len(repos) != 1 || repos[0] != resolvedRepo {
		t.Fatalf("repos: %v, %v", repos, err)
	}
	if _, err := worktree.ListFast(t.TempDir()); err == nil {
		t.Fatal("expected non-repository error")
	}
	if err := os.WriteFile(filepath.Join(r, "README"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := worktree.List(r)
	if err != nil || len(items) != 1 || !items[0].Dirty || items[0].Changes != 1 {
		t.Fatalf("list: %#v, %v", items, err)
	}
}

func TestStatusAndDetachedWorktree(t *testing.T) {
	if got := worktree.Status(worktree.Item{Changes: 2, Ahead: 1, Behind: 3}); got != "(2 files changed · ahead 1 · behind 3)" {
		t.Fatal(got)
	}
	if got := worktree.Status(worktree.Item{}); got != "(clean)" {
		t.Fatal(got)
	}
	r := repo(t)
	path, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	git(t, path, "checkout", "--detach")
	items, err := worktree.ListFast(r)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Path == path {
			if !item.Detached || item.Branch != "" {
				t.Fatalf("detached item: %#v", item)
			}
			return
		}
	}
	t.Fatalf("detached worktree not found: %#v", items)
}

func TestFindReturnsPrimary(t *testing.T) {
	item, err := worktree.Find(repo(t), "main")
	if err != nil || !item.Primary {
		t.Fatalf("%+v %v", item, err)
	}
}

func TestRepositoryBoundariesAndExistingBranch(t *testing.T) {
	notRepo := t.TempDir()
	if got := worktree.Root(notRepo); got != notRepo {
		t.Fatalf("root: %q", got)
	}
	if _, err := worktree.CurrentRepo(notRepo); err == nil {
		t.Fatal("non-repository accepted")
	}
	r := repo(t)
	sibling := filepath.Join(filepath.Dir(r), "web")
	if err := os.Mkdir(sibling, 0750); err != nil {
		t.Fatal(err)
	}
	git(t, sibling, "init", "-b", "main")
	repos, err := worktree.Repos(r)
	if err != nil || len(repos) != 2 {
		t.Fatalf("repos: %v, %v", repos, err)
	}
	path, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.Remove(r, "AG-1"); err != nil {
		t.Fatal(err)
	}
	if got, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "sibling"}); err != nil || got != path {
		t.Fatalf("existing branch: %q, %v", got, err)
	}
	if _, err := worktree.Find(r, "missing"); err == nil {
		t.Fatal("missing worktree accepted")
	}
}

func TestAddRejectsDetachedPrimaryBeforeCreatingLayout(t *testing.T) {
	r := repo(t)
	git(t, r, "checkout", "--detach")
	if _, err := worktree.Add(r, "AG-1", "main", config.Config{Layout: "inside"}); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("add error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r, ".worktrees")); !os.IsNotExist(err) {
		t.Fatalf("layout was created: %v", err)
	}
}

func TestUpdateRejectsDirtyBeforeFetch(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "README"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := worktree.Update(r, "main"); err == nil || strings.Contains(err.Error(), "fetch") {
		t.Fatal(err)
	}
}

func TestUpdateRejectsNonBaseBranchBeforeFetch(t *testing.T) {
	r := repo(t)
	git(t, r, "checkout", "-b", "feature")
	if err := worktree.Update(r, "main"); err == nil || strings.Contains(err.Error(), "fetch") {
		t.Fatal(err)
	}
}

func TestCheckoutBaseRejectsDirtyRoot(t *testing.T) {
	r := repo(t)
	git(t, r, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(r, "README"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := worktree.CheckoutBase(r, "main"); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("checkout error: %v", err)
	}
}

func TestCheckoutBaseChecksOutBase(t *testing.T) {
	r := repo(t)
	git(t, r, "checkout", "-b", "feature")
	if err := worktree.CheckoutBase(r, "main"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = r
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "main" {
		t.Fatalf("branch: %q, %v", out, err)
	}
}

func TestRootManagementRejectsDetachedRoot(t *testing.T) {
	r := repo(t)
	git(t, r, "checkout", "--detach")
	for _, operation := range []func(string) error{
		func(path string) error { return worktree.CheckoutBase(path, "main") },
		worktree.Discard,
	} {
		if err := operation(r); err == nil || !strings.Contains(err.Error(), "detached") {
			t.Fatalf("operation error: %v", err)
		}
	}
}

func TestDiscardResetsAndCleansAllChanges(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "README"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "untracked"), []byte("remove"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := worktree.Discard(r); err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(r, "README")) // #nosec G304 -- test reads its own fixed fixture path.
	if err != nil || string(readme) != "ok" {
		t.Fatalf("README: %q, %v", readme, err)
	}
	if _, err := os.Stat(filepath.Join(r, "untracked")); !os.IsNotExist(err) {
		t.Fatalf("untracked file remained: %v", err)
	}
}

func TestUpdateFastForwardsAndRejectsDivergedHistory(t *testing.T) {
	t.Run("fast forward", func(t *testing.T) {
		root, peer := remoteRepos(t)
		if err := os.WriteFile(filepath.Join(peer, "peer"), []byte("peer"), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, peer, "add", ".")
		git(t, peer, "commit", "-m", "peer")
		git(t, peer, "push")
		if err := worktree.Update(root, "main"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, "peer")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("diverged history", func(t *testing.T) {
		root, peer := remoteRepos(t)
		if err := os.WriteFile(filepath.Join(peer, "peer"), []byte("peer"), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, peer, "add", ".")
		git(t, peer, "commit", "-m", "peer")
		git(t, peer, "push")
		if err := os.WriteFile(filepath.Join(root, "root"), []byte("root"), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "root")
		if err := worktree.Update(root, "main"); err == nil || !strings.Contains(err.Error(), "merge --ff-only") {
			t.Fatal(err)
		}
	})
}

func TestWorktreeErrorsAndMaintenance(t *testing.T) {
	r := repo(t)
	c := config.Config{Layout: "sibling"}
	if _, err := worktree.Add(r, "", "main", c); err == nil {
		t.Fatal("empty branch must fail")
	}
	if _, err := worktree.Add(r, "AG-1", "main", c); err != nil {
		t.Fatal(err)
	}
	if err := worktree.Remove(r, "missing"); err == nil {
		t.Fatal("missing worktree must fail")
	}
	if err := worktree.Prune(r); err != nil {
		t.Fatal(err)
	}
	if err := worktree.Fetch(r, "main"); err == nil {
		t.Fatal("fetch without origin must fail")
	}
	if err := worktree.Update(r, "main"); err == nil {
		t.Fatal("update without origin must fail")
	}
}
