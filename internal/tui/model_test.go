package tui

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

func TestDeleteNeedsConfirmation(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/tmp/api.AG-1"}}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if !updated.(Model).confirm {
		t.Fatal("delete must require confirmation")
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

func TestHighlightRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := highlight("row"); got != "row" {
		t.Fatalf("got %q", got)
	}
}
