package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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

func TestClickOnAnotherGroupSwitchesSelection(t *testing.T) {
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"},
		{Repo: "web", Branch: "AG-1", Path: "/web.AG-1"},
		{Repo: "api", Branch: "AG-2", Path: "/api.AG-2"},
	})
	m = resizeSmall(m)
	m = click(m, bulletColumn, 1) // AG-1 header is line 0
	m = click(m, bulletColumn, 4) // AG-2 header is line 3
	if m.cursor != 2 || m.feature != "AG-2" || m.selected["/api.AG-1"] || m.selected["/web.AG-1"] || !m.selected["/api.AG-2"] {
		t.Fatalf("click did not switch groups: cursor %d, %#v", m.cursor, m.selected)
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

// clickText clicks the first screen cell of text, failing if it is not on
// screen. It exercises the same rendered view the user sees.
func clickText(t *testing.T, m Model, text string) (Model, tea.Cmd) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if x := strings.Index(line, text); x >= 0 {
			updated, cmd := m.Update(tea.MouseClickMsg{X: ansi.StringWidth(line[:x]), Y: y, Button: tea.MouseLeft})
			return updated.(Model), cmd
		}
	}
	t.Fatalf("%q not on screen: %q", text, ansi.Strip(m.View().Content))
	return m, nil
}

func selectedRoot() Model {
	m := featureAndRoots()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updated.(Model)
	return click(m, bulletColumn, 4)
}

func TestClickCommandsHintOpensPaletteAndItemRunsIt(t *testing.T) {
	m := selectedRoot()
	m, _ = clickText(t, m, "[enter] commands")
	if !m.palette {
		t.Fatal("commands hint did not open the palette")
	}
	m, _ = clickText(t, m, "create worktree")
	if m.palette || !m.input {
		t.Fatalf("palette item did not run: palette %v input %v", m.palette, m.input)
	}
	m, _ = clickText(t, m, "[esc] cancel")
	if m.input || !m.selected["/api"] {
		t.Fatalf("cancel hint: input %v, selection %v", m.input, m.selected)
	}
}

func TestClickPaletteCloseKeepsSelection(t *testing.T) {
	m := selectedRoot()
	m = press(m, "enter")
	m, _ = clickText(t, m, "[esc] close")
	if m.palette || !m.selected["/api"] {
		t.Fatalf("close hint: palette %v, selection %v", m.palette, m.selected)
	}
}

func TestClickPaletteItemFocusesBeforeRunning(t *testing.T) {
	m := selectedRoot()
	m = press(m, "enter")
	m, _ = clickText(t, m, "remove all worktrees")
	if !m.confirm || m.pending != actionRemoveAll || m.pCursor != 1 {
		t.Fatalf("remove item: confirm %v pending %q pCursor %d", m.confirm, m.pending, m.pCursor)
	}
	m, _ = clickText(t, m, "[esc/n] cancel")
	if m.confirm {
		t.Fatal("cancel hint did not close the confirmation")
	}
	m = press(m, "enter")
	m, _ = clickText(t, m, "remove all worktrees")
	m, cmd := clickText(t, m, "[enter/y] confirm")
	if m.confirm || m.busy != actionRemoveAll || cmd == nil {
		t.Fatalf("confirm hint: confirm %v busy %q", m.confirm, m.busy)
	}
}

func TestClickClearHintClearsSelectionAndQuitHintQuits(t *testing.T) {
	m := selectedRoot()
	m, _ = clickText(t, m, "[esc] clear")
	if len(m.selectedRoots()) != 0 {
		t.Fatalf("clear hint kept the selection: %v", m.selected)
	}
	if strings.Contains(m.View().Content, "clear") {
		t.Fatal("clear hint shown without a selection")
	}
	_, cmd := clickText(t, m, "[q/esc] quit")
	if cmd == nil {
		t.Fatal("quit hint did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit hint did not return quit command")
	}
}

func TestClickBesideHintsDoesNothing(t *testing.T) {
	m := selectedRoot()
	m, _ = clickText(t, m, "loading")
	if m.palette || !m.selected["/api"] {
		t.Fatalf("click on the status text acted: palette %v", m.palette)
	}
}
