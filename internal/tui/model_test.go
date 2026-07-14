package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

func tuiRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "api")
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
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	return dir
}

func TestDeleteNeedsConfirmation(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/tmp/api.AG-1"}}
	m.group = "AG-1"
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if !updated.(Model).confirm {
		t.Fatal("delete must require confirmation")
	}
}

func TestEnterConfirmsDelete(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/tmp/api.AG-1"}}
	m.group = "AG-1"
	m.confirm = true
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.(Model).confirm {
		t.Fatal("enter must confirm deletion")
	}
}

func TestStyleRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := style("1", "text"); got != "text" {
		t.Fatalf("got %q", got)
	}
	os.Unsetenv("NO_COLOR")
	if got := repoColor("api"); got == "" {
		t.Fatal("missing repo color")
	}
}

func TestSelectionUsesWholeBranchGroup(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1"}, {Repo: "web", Branch: "AG-1"}, {Repo: "api", Branch: "AG-2"}}
	m.cursor = 1
	m.group = "AG-1"
	if got := m.selectedBranch(); got != "AG-1" {
		t.Fatalf("got %q", got)
	}
	if got := m.groupSize(m.selectedBranch()); got != 2 {
		t.Fatalf("got %d", got)
	}
}

func TestItemStatusUsesWords(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := itemStatus(worktree.Item{Changes: 2, Ahead: 1, Behind: 3})
	if got != "(2 files changed · ahead 1 · behind 3)" {
		t.Fatalf("got %q", got)
	}
}

func TestSelectionCountsNameTheirAction(t *testing.T) {
	if got := worktreeCount(1); got != "1 worktree" {
		t.Fatalf("got %q", got)
	}
	if got := projectCount(2); got != "2 projects" {
		t.Fatalf("got %q", got)
	}
}

func TestHighlightRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := highlight("row"); got != "row" {
		t.Fatalf("got %q", got)
	}
}

func TestHelpNeedsSelection(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := New("/tmp", config.Config{})
	if strings.Contains(m.View().Content, "remove group") {
		t.Fatal("help shown without selection")
	}
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1"}}
	m.group = "AG-1"
	if !strings.Contains(m.View().Content, "remove group") {
		t.Fatal("missing selected help")
	}
}

func TestSelectedGroupHelpOrder(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := New("/tmp", config.Config{BaseBranch: "main"})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1"}}
	m.group = "AG-1"
	view := m.View().Content
	for _, text := range []string{"Enter shell", "esc cancel", "e editor", "a agent", "d remove group", "p prune", "u update main", "q quit"} {
		if !strings.Contains(view, text) {
			t.Fatalf("missing %q", text)
		}
	}
	_, update := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if update == nil {
		t.Fatal("u must update the selected group")
	}
}

func TestPrimaryProjectsStartUnselected(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "main", Path: "/tmp/api", Primary: true}}
	m.projects = map[string]bool{"/tmp/api": false}
	if m.projectCount() != 0 {
		t.Fatal("primary project starts selected")
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if updated.(Model).projectCount() != 1 {
		t.Fatal("space must select primary project")
	}
}

func TestArrowsDoNotSelectWorktreeGroup(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1"}, {Repo: "web", Branch: "AG-2"}}
	updated, _ := m.Update(tea.KeyPressMsg{Text: "j"})
	m = updated.(Model)
	if m.selectedBranch() != "" {
		t.Fatal("arrow navigation selected a group")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if updated.(Model).selectedBranch() != "AG-2" {
		t.Fatal("space must select focused group")
	}
}

func TestEscapeClearsSelections(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.group = "AG-1"
	m.projects = map[string]bool{"/tmp/api": true}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.group != "" || m.projectCount() != 0 {
		t.Fatal("escape must clear selections")
	}
}

func TestWorktreeGroupMustBeClearedBeforeChanging(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1"}, {Repo: "web", Branch: "AG-2"}}
	m.group = "AG-1"
	m.cursor = 1
	updated, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if got := updated.(Model).group; got != "AG-1" {
		t.Fatalf("got %q", got)
	}
}

func TestFeaturePrimaryIsWorktreeGroup(t *testing.T) {
	m := New("/tmp", config.Config{BaseBranch: "main"})
	m.items = []worktree.Item{{Branch: "feature", Path: "/tmp/api", Primary: true}}
	m.projects = map[string]bool{}
	updated, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = updated.(Model)
	if m.group != "feature" || m.projectCount() != 0 {
		t.Fatal("feature primary checkout must be a worktree group")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if updated.(Model).confirm {
		t.Fatal("primary checkout group must not be removable")
	}
}

func TestDisplayPathUsesWorktreeDirectory(t *testing.T) {
	if got := displayPath("/Users/me/dev/api.AG-1"); got != "api.AG-1" {
		t.Fatalf("got %q", got)
	}
}

func TestShellCommandUsesWorktreeDirectory(t *testing.T) {
	if got := shellCommand("/tmp/worktree").Dir; got != "/tmp/worktree" {
		t.Fatalf("got %q", got)
	}
}

func TestTypeBranchAndActiveItem(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.input = true
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	m = updated.(Model)
	if m.branch != "A1" {
		t.Fatalf("branch: %q", m.branch)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updated.(Model)
	if m.branch != "A" {
		t.Fatalf("backspace: %q", m.branch)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if updated.(Model).input {
		t.Fatal("escape must leave branch input")
	}
	m.input, m.branch = true, ""
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.(Model).input {
		t.Fatal("enter must leave empty branch input")
	}
	m.input, m.group = false, "AG-1"
	m.items = []worktree.Item{{Branch: "AG-1", Path: "/tmp/one"}, {Branch: "AG-2", Path: "/tmp/two"}}
	m.cursor = 1
	if item, ok := m.activeItem(); !ok || item.Path != "/tmp/one" {
		t.Fatalf("active: %#v, %t", item, ok)
	}
	m.group = ""
	if _, ok := m.activeItem(); ok {
		t.Fatal("empty group has no active item")
	}
}

func TestUpdateStatusAndCommands(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	m := New("/tmp", config.Config{})
	updated, _ := m.Update(loaded{items: []worktree.Item{{Repo: "api", Branch: "main", Path: "/tmp/api", Primary: true}}})
	m = updated.(Model)
	if m.message != "1 worktrees (checking status…)" || m.projects["/tmp/api"] {
		t.Fatalf("loaded: %#v", m)
	}
	updated, _ = m.Update(loaded{err: os.ErrNotExist})
	if updated.(Model).message != os.ErrNotExist.Error() {
		t.Fatal("load error not shown")
	}
	msg := run("", "/tmp")()
	if result, ok := msg.(operationResult); !ok || result.err == nil {
		t.Fatalf("run: %#v", msg)
	}
	if shellCommand("/tmp").Path == "" {
		t.Fatal("missing shell command")
	}
	if got := style("1", "text"); got == "text" {
		t.Fatal("color style missing")
	}
	if got := highlight("text"); got == "text" {
		t.Fatal("highlight missing")
	}
}

func TestKeyboardCommands(t *testing.T) {
	m := New("/tmp", config.Config{})
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if updated.(Model).message == "" || cmd == nil {
		t.Fatal("quit command missing")
	}
	m = New("/tmp", config.Config{})
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if updated.(Model).message != "select primary projects with space first" {
		t.Fatal("new branch needs projects")
	}
	m.projects = map[string]bool{"/tmp/api": true}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if !updated.(Model).input {
		t.Fatal("new branch input missing")
	}
	m = New("/tmp", config.Config{})
	m.confirm = true
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if updated.(Model).confirm {
		t.Fatal("confirmation must cancel")
	}
	m = New("/tmp", config.Config{})
	m.items = []worktree.Item{{Branch: "AG-1", Path: "/tmp/one"}}
	m.group = "AG-1"
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'e', Text: "e"}, {Code: 'a', Text: "a"}, {Code: 'p', Text: "p"}, {Code: 'u', Text: "u"}} {
		if _, cmd := m.Update(key); cmd == nil {
			t.Fatalf("missing command for %s", key.String())
		}
	}
}

func TestLoadedDetailedResultWins(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.detailed = true
	m.message = "detailed"
	updated, _ := m.Update(loaded{items: []worktree.Item{{Branch: "old"}}})
	if updated.(Model).message != "detailed" {
		t.Fatal("fast result replaced detailed state")
	}
}

func TestLoadAndOperationCommands(t *testing.T) {
	dir := tuiRepo(t)
	m := New(dir, config.Config{BaseBranch: "main"})
	for _, detailed := range []bool{false, true} {
		msg := m.load(detailed)()
		result, ok := msg.(loaded)
		if !ok || result.err != nil || len(result.items) != 1 || result.detailed != detailed {
			t.Fatalf("load: %#v", msg)
		}
	}
	m.items = []worktree.Item{{Repo: "api", Branch: "main", Path: dir, Primary: true}}
	m.projects = map[string]bool{dir: true}
	m.group = "main"
	msg := m.updateGroup()()
	if result, ok := msg.(operationResult); !ok || result.err == nil {
		t.Fatalf("update: %#v", msg)
	}
	m.input, m.branch = true, "AG-1"
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(updated.(Model).message, "invalid reference") {
		t.Fatalf("create error: %q", updated.(Model).message)
	}
}
