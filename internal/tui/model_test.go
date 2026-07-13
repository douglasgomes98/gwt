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
