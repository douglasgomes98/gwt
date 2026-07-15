package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func TestV0CLIRejectsRemovedCompatibilityForms(t *testing.T) {
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, testRepo(t), "", config.Config{Layout: "sibling", BaseBranch: "main"})
	for _, args := range [][]string{{"open", "AG-1", "-p"}, {"prune", "extra"}, {"version", "extra"}} {
		if err := a.Run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
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

func TestListDisplaysDetachedWorktree(t *testing.T) {
	dir := testRepo(t)
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "sibling", BaseBranch: "main"})
	if err := a.Run([]string{"add", "AG-1"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".AG-1")
	cmd := exec.Command("git", "checkout", "--detach")
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, out)
	}
	var out bytes.Buffer
	a.Out = &out
	if err := a.Run([]string{"list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), path+"\t(detached)\t(clean)") {
		t.Fatalf("list output: %q", out.String())
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

func TestOpenConfiguredCommandsAndUsageErrors(t *testing.T) {
	dir := testRepo(t)
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "sibling", BaseBranch: "main", Editor: "true", Agent: "true"})
	if err := a.Run([]string{"add", "AG-1"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"open", "AG-1", "-e"}, {"open", "AG-1", "-a"}} {
		if err := a.Run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{{"open", "AG-1", "-e", "-a"}, {"open", "missing"}, {"rm", "AG-1", "--all", "extra"}, {"update", "extra"}} {
		if err := a.Run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestRemoveAllRequiresAtLeastOneMatch(t *testing.T) {
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, testRepo(t), "", config.Config{Layout: "sibling", BaseBranch: "main"})
	if err := a.Run([]string{"rm", "missing", "--all"}); err == nil {
		t.Fatal("rm --all accepted a missing branch")
	}
}

func TestAddRejectsDetachedPrimary(t *testing.T) {
	dir := testRepo(t)
	cmd := exec.Command("git", "checkout", "--detach")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, out)
	}
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{Layout: "inside", BaseBranch: "main"})
	if err := a.Run([]string{"add", "AG-1"}); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("add error: %v", err)
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

func TestCheckoutBaseAndDiscardCommands(t *testing.T) {
	dir := testRepo(t)
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, out)
	}
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, dir, "", config.Config{BaseBranch: "main"})
	if err := a.Run([]string{"checkout-base"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.Run([]string{"discard"}); err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(dir, "README"))
	if err != nil || string(readme) != "ok" {
		t.Fatalf("README: %q, %v", readme, err)
	}
}

func TestRootManagementCommandsRejectArguments(t *testing.T) {
	a := New(&bytes.Buffer{}, &bytes.Buffer{}, testRepo(t), "", config.Config{})
	for _, args := range [][]string{{"checkout-base", "extra"}, {"discard", "extra"}} {
		if err := a.Run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestCommandErrorsAndHelpers(t *testing.T) {
	a := App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Dir: t.TempDir()}
	for _, args := range [][]string{nil, {"unknown"}, {"--version"}, {"-version"}, {"help", "add"}, {"add"}, {"open"}, {"rm"}, {"list", "extra"}, {"upgrade", "extra"}} {
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

func TestUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name       string
		executable string
		prefix     string
		wantName   string
		wantArgs   []string
		fail       bool
	}{
		{"go", "/tmp/gwt", "", "go", []string{"install", "github.com/douglasgomes98/gwt/cmd/gwt@latest"}, false},
		{"homebrew", "/tmp/homebrew/bin/gwt", "/tmp/homebrew", "brew", []string{"upgrade", "gwt"}, false},
		{"updater failure", "/tmp/gwt", "", "go", []string{"install", "github.com/douglasgomes98/gwt/cmd/gwt@latest"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldExecutable, oldCommand := executable, command
			t.Cleanup(func() { executable, command = oldExecutable, oldCommand })
			executable = func() (string, error) { return tc.executable, nil }
			command = func(name string, args ...string) *exec.Cmd {
				if name == "brew" && slices.Equal(args, []string{"--prefix", "gwt"}) {
					if tc.prefix == "" {
						return exec.Command("false")
					}
					return exec.Command("sh", "-c", "printf "+tc.prefix)
				}
				if name == tc.wantName && slices.Equal(args, tc.wantArgs) {
					if tc.fail {
						return exec.Command("false")
					}
					return exec.Command("true")
				}
				t.Fatalf("command = %s %v", name, args)
				return nil
			}
			err := New(io.Discard, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"upgrade"})
			if tc.fail && (err == nil || !strings.Contains(err.Error(), "upgrade")) {
				t.Fatalf("error = %v", err)
			}
			if !tc.fail && err != nil {
				t.Fatal(err)
			}
		})
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
