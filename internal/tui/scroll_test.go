package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes98/gwt/internal/worktree"
)

const manyItemsCount = 30

// manyItems returns enough root worktree items to overflow a small terminal.
func manyItems() []worktree.Item {
	items := make([]worktree.Item, 0, manyItemsCount)
	for i := range manyItemsCount {
		repo := "repo" + strconv.Itoa(i)
		items = append(items, worktree.Item{Repo: repo, Branch: "main", Path: "/" + repo, Primary: true})
	}
	return items
}

const smallTerminalHeight = 10

func resizeSmall(m Model) Model {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: smallTerminalHeight})
	return updated.(Model)
}

func TestViewClipsRowsToFitSmallTerminal(t *testing.T) {
	m := modelWith(manyItems())
	m = resizeSmall(m)
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) > smallTerminalHeight {
		t.Fatalf("view has %d lines, want at most %d: %q", len(lines), smallTerminalHeight, m.View().Content)
	}
	// The footer (status/key hints) must still be present even though the
	// list overflows the terminal.
	if !strings.Contains(m.View().Content, "quit") {
		t.Fatalf("footer missing from clipped view: %q", m.View().Content)
	}
}

func TestViewScrollsToKeepCursorVisible(t *testing.T) {
	m := modelWith(manyItems())
	m = resizeSmall(m)
	topRepo := m.items[0].Repo
	for range len(m.items) - 1 {
		m = press(m, "down")
	}
	lastRepo := m.items[m.cursor].Repo
	view := m.View().Content
	if !strings.Contains(view, lastRepo+" ") {
		t.Fatalf("cursor row (%s) scrolled out of view: %q", lastRepo, view)
	}
	if strings.Contains(view, topRepo+" ") {
		t.Fatalf("top of list (%s) still shown after scrolling to the end: %q", topRepo, view)
	}
}

func TestPageDownScrollsWithoutMovingCursor(t *testing.T) {
	m := modelWith(manyItems())
	m = resizeSmall(m)
	topRepo := m.items[0].Repo
	before := m.View().Content
	m = press(m, "pgdown")
	after := m.View().Content
	if m.cursor != 0 {
		t.Fatalf("pgdown moved the cursor: %d", m.cursor)
	}
	if before == after {
		t.Fatalf("pgdown did not change the visible rows")
	}
	if strings.Contains(after, topRepo+" ") {
		t.Fatalf("pgdown should scroll %s's row out of view: %q", topRepo, after)
	}
}

func TestMouseWheelScrollsWithoutMovingCursor(t *testing.T) {
	m := modelWith(manyItems())
	m = resizeSmall(m)
	topRepo := m.items[0].Repo
	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Fatalf("mouse wheel moved the cursor: %d", m.cursor)
	}
	if strings.Contains(m.View().Content, topRepo+" ") {
		t.Fatalf("mouse wheel should scroll %s's row out of view: %q", topRepo, m.View().Content)
	}
}

func TestViewWithoutWindowSizeRendersEverythingUnclipped(t *testing.T) {
	m := modelWith(manyItems())
	view := m.View().Content
	for i := range manyItemsCount {
		if !strings.Contains(view, fmt.Sprintf("repo%d ", i)) {
			t.Fatalf("repo%d missing from unclipped view", i)
		}
	}
}
