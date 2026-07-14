package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

func modelWith(items []worktree.Item) Model {
	m := New("/tmp", config.Config{})
	m.items = items
	return m
}

func press(m Model, key string) Model {
	updated, _ := m.Update(tea.KeyPressMsg{Text: key})
	return updated.(Model)
}

func TestFeatureSelectionStartsWholeGroupThenTogglesOne(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.A"}, {Repo: "web", Branch: "AG-1", Path: "/web.A"}})
	m = press(m, "space")
	if !m.selected["/api.A"] || !m.selected["/web.A"] {
		t.Fatal("group not selected")
	}
	m.cursor = 1
	m = press(m, "space")
	if m.selected["/web.A"] || !m.selected["/api.A"] {
		t.Fatal("row toggle failed")
	}
}

func TestRootAndFeatureSelectionsAreExclusive(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}, {Repo: "api", Branch: "AG-1", Path: "/api.A"}})
	m = press(m, "space")
	m.cursor = 1
	m = press(m, "space")
	if len(m.selectedRoots()) != 0 || m.feature != "AG-1" {
		t.Fatal("feature selection must clear roots")
	}
	m.cursor = 0
	m = press(m, "space")
	if m.feature != "" || !m.selected["/api"] || m.selected["/api.A"] {
		t.Fatal("root selection must clear feature")
	}
}

func TestPaletteOnlyShowsValidCLICommands(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.A"}})
	m.config = config.Config{Editor: "code", Agent: "claude"}
	m = press(m, "space")
	if got, want := m.availableActions(), []action{actionOpen, actionOpenEditor, actionOpenAgent, actionRemove, actionPrune}; !slices.Equal(got, want) {
		t.Fatalf("feature actions: %v", got)
	}
	m.config = config.Config{}
	if got, want := m.availableActions(), []action{actionOpen, actionRemove, actionPrune}; !slices.Equal(got, want) {
		t.Fatalf("feature actions without tools: %v", got)
	}
	m = modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m = press(m, "space")
	if got, want := m.availableActions(), []action{actionAdd, actionPrune, actionUpdate}; !slices.Equal(got, want) {
		t.Fatalf("root actions: %v", got)
	}
}

func TestMultipleRootsAndFeaturesUseBatchCommands(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}, {Repo: "web", Branch: "main", Path: "/web", Primary: true}})
	m = press(m, "space")
	m.cursor = 1
	m = press(m, "space")
	if got, want := m.availableActions(), []action{actionAddAll, actionPrune, actionUpdate}; !slices.Equal(got, want) {
		t.Fatalf("root actions: %v", got)
	}
	m = modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.A"}, {Repo: "web", Branch: "AG-1", Path: "/web.A"}})
	m = press(m, "space")
	if got, want := m.availableActions(), []action{actionRemoveAll, actionPrune}; !slices.Equal(got, want) {
		t.Fatalf("feature actions: %v", got)
	}
}

func TestDetachedItemIsVisibleButCannotBeSelected(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "", Path: "/api.detached", Detached: true}})
	m = press(m, "space")
	if len(m.selected) != 0 || m.feature != "" {
		t.Fatal("detached item selected")
	}
	if !strings.Contains(m.View().Content, "api.detached") || !strings.Contains(m.View().Content, "(detached)") {
		t.Fatal("detached item not visible")
	}
}

func TestPaletteNavigationAndEscapePreserveSelection(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.A"}})
	m = press(m, "space")
	m = press(m, "enter")
	if !m.palette {
		t.Fatal("enter did not open palette")
	}
	m = press(m, "j")
	if m.pCursor != 1 {
		t.Fatalf("cursor: %d", m.pCursor)
	}
	m = press(m, "esc")
	if m.palette || !m.selected["/api.A"] {
		t.Fatal("escape must only close the palette")
	}
}

func TestPaletteAddStartsBranchInputWithoutMutation(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m = press(m, "space")
	m = press(m, "enter")
	m = press(m, "enter")
	if m.palette || !m.input || m.branch != "" || m.message != "branch: " {
		t.Fatalf("add state: %#v", m)
	}
}

func TestBranchInputRendersAfterBranchLabel(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m = press(m, "space")
	m, _ = m.execute(actionAdd)
	m = press(m, "A")
	if !strings.Contains(m.View().Content, "branch: A") {
		t.Fatalf("branch input: %q", m.View().Content)
	}
}

func TestDetachedPrimaryRootCannotBeSelected(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true, Detached: true}})
	m = press(m, "space")
	if len(m.selected) != 0 || len(m.selectedRoots()) != 0 {
		t.Fatal("detached primary root selected")
	}
}

func TestDirectActionShortcutsDoNothing(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m = press(m, "space")
	for _, key := range []string{"n", "d", "p", "u", "e", "a"} {
		updated, cmd := m.Update(tea.KeyPressMsg{Text: key})
		m = updated.(Model)
		if cmd != nil || !m.selected["/api"] || m.feature != "" || m.palette {
			t.Fatalf("shortcut %q changed state", key)
		}
	}
}

func TestStyleAndCounts(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := style("1", "text"); got != "text" {
		t.Fatalf("got %q", got)
	}
	if got := highlight("row"); got != "row" {
		t.Fatalf("got %q", got)
	}
	if got := worktreeCount(2); got != "2 worktrees" {
		t.Fatalf("got %q", got)
	}
	if got := itemStatus(worktree.Item{Changes: 2, Ahead: 1, Behind: 3}); got != "(2 files changed · ahead 1 · behind 3)" {
		t.Fatalf("got %q", got)
	}
	os.Unsetenv("NO_COLOR")
	if repoColor("api") == "" {
		t.Fatal("missing repo color")
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

func TestTUIAddUsesOnlySelectedRootsAndNoFetch(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	web := tuiTestRepo(t, parent, "web")
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "main", Path: api, Primary: true},
		{Repo: "web", Branch: "main", Path: web, Primary: true},
	})
	m.selected[api] = true
	m, _ = m.execute(actionAdd)
	if !m.input {
		t.Fatal("add did not start branch input")
	}
	for _, key := range []string{"A", "G", "-", "1"} {
		m = press(m, key)
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Fatal("enter did not add selected roots")
	}
	result := cmd()
	m = updated.(Model)
	m.Update(result)
	if _, err := os.Stat(filepath.Join(parent, "api.AG-1")); err != nil {
		t.Fatalf("selected root not added: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "web.AG-1")); !os.IsNotExist(err) {
		t.Fatalf("unselected root changed: %v", err)
	}
}

func TestTUIRemoveAllUsesOnlySelectedFeatureRows(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	web := tuiTestRepo(t, parent, "web")
	apiFeature, err := worktree.Add(api, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	webFeature, err := worktree.Add(web, "AG-1", "main", config.Config{Layout: "sibling"})
	if err != nil {
		t.Fatal(err)
	}
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "main", Path: api, Primary: true},
		{Repo: "api", Branch: "AG-1", Path: apiFeature},
		{Repo: "web", Branch: "main", Path: web, Primary: true},
		{Repo: "web", Branch: "AG-1", Path: webFeature},
	})
	m.feature = "AG-1"
	m.selected[apiFeature] = true
	m, _ = m.execute(actionRemoveAll)
	if !m.confirm {
		t.Fatal("remove did not request confirmation")
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Fatal("confirmation did not remove selected rows")
	}
	m = updated.(Model)
	m.Update(cmd())
	if _, err := os.Stat(apiFeature); !os.IsNotExist(err) {
		t.Fatalf("selected feature was not removed: %v", err)
	}
	if _, err := os.Stat(webFeature); err != nil {
		t.Fatalf("unselected feature was removed: %v", err)
	}
}

func TestOperationResultClearsSelectionAndReloads(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"}})
	m.feature = "AG-1"
	m.selected["/api.AG-1"] = true
	updated, cmd := m.Update(operationResult{message: "done", reload: true})
	got := updated.(Model)
	if got.feature != "" || len(got.selected) != 0 || cmd == nil {
		t.Fatal("operation did not reset")
	}
}

func TestOperationResultPartialErrorSurvivesReloadMessages(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"}})
	updated, _ := m.Update(operationResult{err: partial(actionAdd, os.ErrPermission), reload: true})
	m = updated.(Model)
	updated, _ = m.Update(loaded{items: m.items, detailed: false})
	m = updated.(Model)
	updated, _ = m.Update(loaded{items: m.items, detailed: true})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "result may be partial") {
		t.Fatalf("operation error lost after reload: %q", m.View().Content)
	}
}

func TestUserInteractionClearsOperationResult(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"}})
	updated, _ := m.Update(operationResult{err: partial(actionAdd, os.ErrPermission)})
	m = press(updated.(Model), "down")
	if strings.Contains(m.View().Content, "result may be partial") {
		t.Fatalf("operation error remained after user input: %q", m.View().Content)
	}
}

func tuiTestRepo(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}, {"commit", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}
