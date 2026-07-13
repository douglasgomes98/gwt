package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

func TestDeleteNeedsConfirmation(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/tmp/api.AG-1"}}
	m.group = "AG-1"
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

func TestPrimaryProjectsStartUnselected(t *testing.T) {
	m := New("/tmp", config.Config{})
	m.items = []worktree.Item{{Repo: "api", Path: "/tmp/api", Primary: true}}
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

func TestDisplayPathUsesWorktreeDirectory(t *testing.T) {
	if got := displayPath("/Users/me/dev/api.AG-1"); got != "api.AG-1" {
		t.Fatalf("got %q", got)
	}
}
