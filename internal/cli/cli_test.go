package cli

import (
	"bytes"
	"errors"
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
	if err := os.Mkdir(dir, 0750); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		cmd := exec.Command("git", args...) // #nosec G204 -- test invokes Git with fixed arguments.
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...) // #nosec G204 -- test invokes Git with fixed arguments.
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
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "PATH") {
		t.Fatalf("list header: %q", out.String())
	}
	if !strings.Contains(lines[0], "BRANCH  STATUS") {
		t.Fatalf("list header: %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], "main    (clean)") {
		t.Fatalf("primary row: %q", lines[1])
	}
	if !strings.HasSuffix(lines[2], "AG-1    (clean)") {
		t.Fatalf("feature row: %q", lines[2])
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

func TestSkillInstallCopiesEmbeddedSkillToSelectedHomes(t *testing.T) {
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	var out bytes.Buffer
	a := New(&out, &bytes.Buffer{}, t.TempDir(), "", config.Config{})
	if err := a.Run([]string{"skill", "install", "--agents", "--claude"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(home, ".agents", "skills", "gwt-worktrees", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "gwt-worktrees", "SKILL.md"),
	} {
		got, err := os.ReadFile(path) // #nosec G304 -- test reads a path built from its temp home.
		if err != nil || !bytes.Equal(got, gwtWorktreesSkill) {
			t.Fatalf("skill at %s = %q, %v", path, got, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("skill mode at %s = %v", path, info.Mode())
		}
	}
	dir, err := os.Stat(filepath.Join(home, ".agents", "skills", "gwt-worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0750 {
		t.Fatalf("skill directory mode = %v", dir.Mode())
	}
}

func TestSkillInstallPreflightsExistingDestination(t *testing.T) {
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	claude := filepath.Join(home, ".claude", "skills", "gwt-worktrees", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(claude), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte("custom"), 0600); err != nil {
		t.Fatal(err)
	}
	err := New(io.Discard, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"skill", "install", "--agents", "--claude"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "gwt-worktrees", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("agents installation = %v", err)
	}
}

func TestSkillInstallReportsHomeLookupFailure(t *testing.T) {
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return "", io.ErrUnexpectedEOF }
	t.Cleanup(func() { userHomeDir = oldHome })

	err := New(io.Discard, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"skill", "install", "--agents"})
	if err == nil || !strings.Contains(err.Error(), "find home directory") {
		t.Fatalf("error = %v", err)
	}
}

func TestSkillInstallReportsDestinationCreationFailure(t *testing.T) {
	oldHome, oldMkdirAll := userHomeDir, makeSkillDir
	userHomeDir = func() (string, error) { return t.TempDir(), nil }
	makeSkillDir = func(string, os.FileMode) error { return io.ErrClosedPipe }
	t.Cleanup(func() {
		userHomeDir = oldHome
		makeSkillDir = oldMkdirAll
	})

	err := New(io.Discard, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"skill", "install", "--agents"})
	if err == nil || !strings.Contains(err.Error(), "create skill destination") {
		t.Fatalf("error = %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestSkillInstallCompletesBeforeReportingFails(t *testing.T) {
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	err := New(failingWriter{}, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"skill", "install", "--agents", "--claude"})
	if err == nil {
		t.Fatalf("error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(home, ".agents", "skills", "gwt-worktrees", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "gwt-worktrees", "SKILL.md"),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("skill after reporting failure = %v", err)
		}
	}
}

func TestSkillInstallRollsBackAfterSecondWriteFails(t *testing.T) {
	home := t.TempDir()
	oldHome, oldOpen := userHomeDir, openSkillFile
	userHomeDir = func() (string, error) { return home, nil }
	calls := 0
	openSkillFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		calls++
		if calls == 2 {
			return nil, io.ErrClosedPipe
		}
		return os.OpenFile(path, flag, perm) // #nosec G304 -- test uses the generated temporary destination.
	}
	t.Cleanup(func() {
		userHomeDir = oldHome
		openSkillFile = oldOpen
	})

	err := New(io.Discard, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"skill", "install", "--agents", "--claude"})
	if err == nil || !strings.Contains(err.Error(), "write skill destination") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".agents", "skills", "gwt-worktrees", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("agents skill after write failure = %v", err)
	}
}

func TestWriteSkillDoesNotOverwriteExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte("custom"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeSkill(path); err == nil {
		t.Fatal("writeSkill overwrote an existing file")
	}
	got, err := os.ReadFile(path) // #nosec G304 -- test reads its temporary fixture.
	if err != nil || string(got) != "custom" {
		t.Fatalf("file = %q, %v", got, err)
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
	if !strings.Contains(out.String(), path) || !strings.Contains(out.String(), "(detached)") || !strings.Contains(out.String(), "(clean)") {
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
	if err := os.WriteFile(filepath.Join(dir, "dirty"), []byte("x"), 0600); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.Run([]string{"discard"}); err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(dir, "README")) // #nosec G304 -- test reads its own fixed fixture path.
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
	for _, args := range [][]string{nil, {"unknown"}, {"--version"}, {"-version"}, {"help", "add"}, {"add"}, {"open"}, {"rm"}, {"list", "extra"}, {"upgrade", "extra"}, {"skill"}, {"skill", "install"}, {"skill", "install", "extra"}, {"skill", "remove", "--agents"}} {
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
					return exec.Command("printf", "%s", tc.prefix) // #nosec G204 -- prefix comes from a fixed test table.
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
	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("add <branch>")) || !strings.Contains(got, "init-config                          Create a local configuration file.") || !strings.Contains(got, "skill install --agents|--claude") || !strings.Contains(got, "upgrade                              Upgrade gwt.") {
		t.Fatalf("unexpected help: %q", got)
	}
}

func TestInitConfigCreatesLocalConfig(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	a := New(&out, &bytes.Buffer{}, dir, "", config.Config{Layout: "inside", BaseBranch: "trunk", Editor: "vim", Agent: "codex"})
	if err := a.Run([]string{"init-config"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "gwt.yml")) // #nosec G304 -- test reads its own fixed fixture path.
	if err != nil {
		t.Fatal(err)
	}
	want := "layout: inside\nbaseBranch: trunk\neditor: vim\nagent: codex\n"
	if got := string(data); got != want {
		t.Fatalf("config = %q, want %q", got, want)
	}
}

func TestInitConfigDoesNotOverwriteExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gwt.yml")
	if err := os.WriteFile(path, []byte("layout: grouped\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := New(io.Discard, io.Discard, dir, "", config.Config{}).Run([]string{"init-config"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- test reads its own fixed fixture path.
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "layout: grouped\n" {
		t.Fatalf("config = %q", got)
	}
}

func TestInitConfigRoundTripsEmptyOptionalCommands(t *testing.T) {
	dir := t.TempDir()
	want := config.Config{Layout: "grouped", BaseBranch: "main", Editor: "", Agent: ""}
	if err := New(io.Discard, io.Discard, dir, "", want).Run([]string{"init-config"}); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestInitConfigRejectsArguments(t *testing.T) {
	err := New(io.Discard, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"init-config", "extra"})
	if err == nil || !strings.Contains(err.Error(), "usage: gwt init-config") {
		t.Fatalf("error = %v", err)
	}
}

func TestInitConfigReportsOutputError(t *testing.T) {
	err := New(failingWriter{}, io.Discard, t.TempDir(), "", config.Config{}).Run([]string{"init-config"})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v", err)
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
