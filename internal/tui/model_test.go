package tui

import (
	"os"
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
	if !strings.Contains(m.View().Content, "api.detached") {
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
