package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes98/gwt/internal/worktree"
)

// bulletColumn is the screen column of the ○/◉ radio in an item row.
const bulletColumn = 2

func click(m Model, x, y int) Model {
	updated, _ := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return updated.(Model)
}

// featureAndRoots renders as:
//
//	0 AG-1 header
//	1 api.AG-1
//	2 web.AG-1
//	3 roots header
//	4 api
//	5 web
func featureAndRoots() Model {
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"},
		{Repo: "web", Branch: "AG-1", Path: "/web.AG-1"},
		{Repo: "api", Branch: "main", Path: "/api", Primary: true},
		{Repo: "web", Branch: "main", Path: "/web", Primary: true},
	})
	return resizeSmall(m)
}

func TestClickOnBulletSelectsLikeSpace(t *testing.T) {
	m := featureAndRoots()
	m = click(m, bulletColumn, 2)
	if m.cursor != 1 {
		t.Fatalf("click did not focus the row: cursor %d", m.cursor)
	}
	if !m.selected["/api.AG-1"] || !m.selected["/web.AG-1"] {
		t.Fatalf("first feature click should select the whole group: %v", m.selected)
	}
	m = click(m, bulletColumn, 2)
	if m.selected["/web.AG-1"] || !m.selected["/api.AG-1"] {
		t.Fatalf("second click should toggle only the clicked row: %v", m.selected)
	}
	m = click(m, bulletColumn, 4)
	if m.cursor != 2 || !m.selected["/api"] || m.feature != "" {
		t.Fatalf("root click should select the root and clear the feature: cursor %d, %v", m.cursor, m.selected)
	}
}

func TestClickNextToBulletStillSelects(t *testing.T) {
	for _, x := range []int{0, 1, 3} {
		m := click(featureAndRoots(), x, 4)
		if !m.selected["/api"] {
			t.Fatalf("click at column %d did not select", x)
		}
	}
}

func TestClickOnRowOnlyMovesCursor(t *testing.T) {
	m := click(featureAndRoots(), 10, 5)
	if m.cursor != 3 {
		t.Fatalf("cursor = %d, want 3", m.cursor)
	}
	if len(m.selectedRoots())+len(m.selectedFeatureItems()) != 0 {
		t.Fatalf("row click selected something: %v", m.selected)
	}
}

func TestClickIgnoresHeadersFooterAndOtherButtons(t *testing.T) {
	m := featureAndRoots()
	m.cursor = 1
	for _, y := range []int{0, 3, smallTerminalHeight - 1} {
		m = click(m, bulletColumn, y)
	}
	updated, _ := m.Update(tea.MouseClickMsg{X: bulletColumn, Y: 4, Button: tea.MouseRight})
	m = updated.(Model)
	if m.cursor != 1 || len(m.selectedRoots())+len(m.selectedFeatureItems()) != 0 {
		t.Fatalf("ignored click changed state: cursor %d, %v", m.cursor, m.selected)
	}
}

func TestClickFollowsScrollOffset(t *testing.T) {
	m := resizeSmall(modelWith(manyItems()))
	m = press(m, "end")
	offset := m.viewport.YOffset()
	m = click(m, bulletColumn, 1)
	want := offset // line 0 is the roots header, so line n is item n-1
	if m.cursor != want || !m.selected[m.items[want].Path] {
		t.Fatalf("cursor = %d, want %d (offset %d)", m.cursor, want, offset)
	}
}

func TestClickIsIgnoredWhileOverlayIsOpen(t *testing.T) {
	m := featureAndRoots()
	// Tall enough that row 5 stays inside the list while the palette is open.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = updated.(Model)
	m = click(m, bulletColumn, 4)
	m = press(m, "enter")
	m = click(m, bulletColumn, 5)
	if m.selected["/web"] || m.cursor != 2 {
		t.Fatalf("click went through the palette: cursor %d, %v", m.cursor, m.selected)
	}
	m.palette, m.confirm = false, true
	m = click(m, bulletColumn, 5)
	m.confirm, m.input = false, true
	m = click(m, bulletColumn, 5)
	if m.selected["/web"] || m.cursor != 2 {
		t.Fatalf("click went through an overlay: cursor %d, %v", m.cursor, m.selected)
	}
}

func TestClickClearsLastResult(t *testing.T) {
	m := featureAndRoots()
	m.result = "removed 2 worktrees"
	m = click(m, 10, 4)
	if m.result != "" || strings.Contains(m.View().Content, "removed 2 worktrees") {
		t.Fatalf("click kept the previous result: %q", m.result)
	}
}
