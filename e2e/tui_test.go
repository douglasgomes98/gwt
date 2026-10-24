//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// The scenarios below drive the real gwt binary in a pseudo-terminal. The
// group they run on renders, at 90×16, as:
//
//	 0 AG-1  3 worktrees      (repo01..03.AG-1)
//	 4 AG-2  2 worktrees      (repo04..05.AG-2)
//	 7 roots  25 roots        (repo01..25, on main)
//	   …
//	14 [q/esc] quit
const width, height = 90, 16

// bullet is the screen column of a row's ○/◉ radio.
const bullet = 2

func standardGroup(t *testing.T) string {
	return group(t, 25, map[string][]int{"AG-1": {1, 2, 3}, "AG-2": {4, 5}})
}

func selectedCount(term *terminal) int {
	return strings.Count(strings.Join(term.Screen(), "\n"), "◉")
}

func TestE2EPageKeysMoveFocusAndKeepIt(t *testing.T) {
	term := startTUI(t, standardGroup(t), width, height)
	term.Press("pgdown", focusOn("repo09", "main"))
	term.Press("down", focusOn("repo10", "main"))
	if term.Row("AG-1") >= 0 {
		t.Fatalf("down after pgdown jumped back to the top:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.Press("end", focusOn("repo25", "main"))
	term.Press("ctrl+u", focusOn("repo12", "main"))
	term.Press("home", focusOn("repo01.AG-1"))
	if term.Row("AG-1", "3 worktrees") != 0 {
		t.Fatalf("home hid the first group header:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.Press("ctrl+d", focusOn("repo09", "main"))
	term.Press("pgup", focusOn("repo01.AG-1"))
}

func TestE2EClickSelectsExactlyLikeSpace(t *testing.T) {
	term := startTUI(t, standardGroup(t), width, height)
	term.Click(bullet, term.Row("repo02.AG-1"), shows("AG-1", "3 worktrees selected"))
	if y, _ := term.Focused(); y != term.Row("repo02.AG-1") {
		t.Fatalf("click did not focus the clicked row:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.Click(bullet, term.Row("repo04.AG-2"), shows("AG-2", "2 worktrees selected"))
	if selectedCount(term) != 2 {
		t.Fatalf("switching groups kept AG-1 selected:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.Click(bullet, term.Row("repo04.AG-2"), shows("AG-2", "1 worktree selected"))
	term.Press("space", shows("AG-2", "2 worktrees selected"))
	term.Click(40, term.Row("repo01", "main"), focusOn("repo01", "main"))
	if !strings.Contains(strings.Join(term.Screen(), "\n"), "2 worktrees selected") {
		t.Fatalf("a click beside the bullet changed the selection:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.Click(bullet, term.Row("repo01", "main"), func(s string) bool { return !strings.Contains(s, "selected") })
	if selectedCount(term) != 1 {
		t.Fatalf("selecting a root did not clear the feature:\n%s", strings.Join(term.Screen(), "\n"))
	}
}

func TestE2EClickAfterWheelHitsTheRowUnderThePointer(t *testing.T) {
	term := startTUI(t, standardGroup(t), width, height)
	for range 3 {
		before := strings.Join(term.Screen(), "\n")
		term.Wheel(true, bullet, 5, func(s string) bool { return s != before })
	}
	if y, _ := term.Focused(); y >= 0 {
		t.Fatalf("the wheel should scroll without carrying the focus along:\n%s", strings.Join(term.Screen(), "\n"))
	}
	target := term.Screen()[3]
	repo := strings.Fields(target)[1]
	term.Click(bullet, 3, focusOn(repo, "◉"))
	if term.Screen()[3] != strings.Replace(strings.Replace(target, "○", "◉", 1), " ", "›", 1) {
		t.Fatalf("clicked row 3 (%q) but the screen shows:\n%s", target, strings.Join(term.Screen(), "\n"))
	}
}

func TestE2EClickAfterResize(t *testing.T) {
	term := startTUI(t, standardGroup(t), width, height)
	term.Resize(50, 10, func(s string) bool { return strings.Count(s, "\n") <= 9 && strings.Contains(s, "quit") })
	term.Click(bullet, term.Row("repo02.AG-1"), shows("AG-1", "3 worktrees selected"))
}

func TestE2EMouseOnlyCreateAndRemove(t *testing.T) {
	dir := standardGroup(t)
	term := startTUI(t, dir, width, height)
	term.Click(bullet, term.Row("repo01", "main"), shows("[enter] commands"))
	term.Click(bullet, term.Row("repo02", "main"), focusOn("repo02", "◉"))
	term.ClickText("[enter] commands", shows("create worktrees"))
	term.ClickText("create worktrees", shows("branch:", "[enter] create"))
	term.Press("AG-9", shows("branch: AG-9"))
	term.ClickText("[enter] create", shows("added 2 worktrees"))
	term.waitFor(shows("repo02.AG-9")) // the list reloads after the result shows
	for _, repo := range []string{"repo01", "repo02"} {
		if !strings.Contains(git(t, dir+"/"+repo, "worktree", "list"), "[AG-9]") {
			t.Fatalf("%s has no AG-9 worktree", repo)
		}
	}

	term.Click(bullet, term.Row("repo01.AG-9"), shows("AG-9", "2 worktrees selected"))
	term.ClickText("[enter] commands", shows("remove worktrees"))
	term.ClickText("remove worktrees", shows("remove selected worktrees?"))
	term.ClickText("[esc/n] cancel", shows("[enter] commands"))
	if !strings.Contains(strings.Join(term.Screen(), "\n"), "2 worktrees selected") {
		t.Fatal("cancelling the removal cleared the selection")
	}
	term.ClickText("[enter] commands", shows("remove worktrees"))
	term.ClickText("remove worktrees", shows("remove selected worktrees?"))
	term.ClickText("[enter/y] confirm", shows("removed 2 worktrees"))
	for _, repo := range []string{"repo01", "repo02"} {
		if strings.Contains(git(t, dir+"/"+repo, "worktree", "list"), "[AG-9]") {
			t.Fatalf("%s still has the AG-9 worktree", repo)
		}
	}
}

func TestE2EPaletteCloseClearAndQuitHints(t *testing.T) {
	term := startTUI(t, standardGroup(t), width, height)
	term.Click(bullet, term.Row("repo02.AG-1"), shows("[enter] commands"))
	term.ClickText("[enter] commands", shows("[esc] close"))
	term.ClickText("[esc] close", shows("[esc] clear"))
	term.ClickText("[esc] clear", shows("[q/esc] quit"))
	if selectedCount(term) != 0 {
		t.Fatalf("clear hint kept the selection:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.ClickText("[q/esc] quit", func(string) bool { return true })
	term.WaitExit()
}

func TestE2ERowClicksAreIgnoredUnderThePalette(t *testing.T) {
	term := startTUI(t, standardGroup(t), width, 30)
	term.Click(bullet, term.Row("repo01", "main"), shows("[enter] commands"))
	term.Press("enter", shows("[esc] close"))
	term.Click(bullet, term.Row("repo02", "main"), shows("[esc] close"))
	term.Press("esc", shows("[esc] clear"))
	if selectedCount(term) != 1 || term.Row("repo02", "◉") >= 0 {
		t.Fatalf("a row click went through the palette:\n%s", strings.Join(term.Screen(), "\n"))
	}
}
