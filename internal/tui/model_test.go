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

func TestFeatureSelectionRequiresEscapeBeforeChangingGroups(t *testing.T) {
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"},
		{Repo: "api", Branch: "AG-2", Path: "/api.AG-2"},
	})
	m = press(m, "space")
	m.cursor = 1
	m = press(m, "space")
	if m.feature != "AG-1" || !m.selected["/api.AG-1"] || m.selected["/api.AG-2"] {
		t.Fatalf("feature changed without escape: %#v", m)
	}
}

func TestEscapeClearsFeatureSelectionBeforeSelectingAnotherFeature(t *testing.T) {
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "main", Path: "/api", Primary: true},
		{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"},
		{Repo: "api", Branch: "AG-2", Path: "/api.AG-2"},
	})
	m.cursor = 1
	m = press(m, "space")
	m = press(m, "esc")
	m.cursor = 2
	m = press(m, "space")
	if m.feature != "AG-2" || m.selected["/api.AG-1"] || !m.selected["/api.AG-2"] {
		t.Fatalf("escape did not clear feature: %#v", m)
	}
	m = press(m, "esc")
	m.cursor = 0
	m = press(m, "space")
	if m.feature != "" || !m.selected["/api"] {
		t.Fatalf("escape did not permit root selection: %#v", m)
	}
}

func TestFeatureSelectionClearsWhenLastItemIsDeselected(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"}})
	m = press(m, "space")
	m = press(m, "space")
	if m.feature != "" || len(m.selectedFeatureItems()) != 0 {
		t.Fatalf("empty feature selection remained active: %#v", m)
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
	if got, want := m.availableActions(), []action{actionAdd, actionPrune, actionUpdate, actionCheckoutBase, actionDiscard}; !slices.Equal(got, want) {
		t.Fatalf("root actions: %v", got)
	}
}

func TestMultipleRootsAndFeaturesUseBatchCommands(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}, {Repo: "web", Branch: "main", Path: "/web", Primary: true}})
	m = press(m, "space")
	m.cursor = 1
	m = press(m, "space")
	if got, want := m.availableActions(), []action{actionAddAll, actionPrune, actionUpdate, actionCheckoutBase, actionDiscard}; !slices.Equal(got, want) {
		t.Fatalf("root actions: %v", got)
	}
	m = modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.A"}, {Repo: "web", Branch: "AG-1", Path: "/web.A"}})
	m = press(m, "space")
	if got, want := m.availableActions(), []action{actionRemoveAll, actionPrune}; !slices.Equal(got, want) {
		t.Fatalf("feature actions: %v", got)
	}
}

func TestViewGroupsPrimaryCheckoutsUnderRoots(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := modelWith([]worktree.Item{
		{Repo: "guru", Branch: "feature", Path: "/guru", Primary: true},
		{Repo: "api", Branch: "feature", Path: "/api.feature"},
	})
	view := m.View().Content
	if !strings.Contains(view, "roots") || strings.Index(view, "feature") > strings.Index(view, "roots") {
		t.Fatalf("unexpected groups: %q", view)
	}
}

func TestDiscardActionRequiresConfirmation(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "guru", Branch: "feature", Path: "/guru", Primary: true}})
	m = press(m, "space")
	m, _ = m.execute(actionDiscard)
	if !m.confirm || m.pending != actionDiscard {
		t.Fatalf("discard state: %#v", m)
	}
	m = press(m, "n")
	if m.confirm {
		t.Fatal("discard confirmation was not cancelled")
	}
}

func TestCheckoutBaseSelectedRoots(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = api
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, out)
	}
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "feature", Path: api, Primary: true}})
	m.selected[api] = true
	result := m.checkoutBaseSelectedRoots()().(operationResult)
	if result.err != nil || !result.reload {
		t.Fatalf("checkout result: %#v", result)
	}
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = api
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "main" {
		t.Fatalf("branch: %q, %v", out, err)
	}
}

func TestDiscardSelectedRoots(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	if err := os.WriteFile(filepath.Join(api, "untracked"), []byte("remove"), 0644); err != nil {
		t.Fatal(err)
	}
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: api, Primary: true}})
	m.selected[api] = true
	result := m.discardSelectedRoots()().(operationResult)
	if result.err != nil || !result.reload {
		t.Fatalf("discard result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(api, "untracked")); !os.IsNotExist(err) {
		t.Fatalf("untracked file remained: %v", err)
	}
}

func TestRootManagementActionsDispatchAndLabel(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m.selected["/api"] = true
	m, cmd := m.execute(actionCheckoutBase)
	if cmd == nil || m.busy != actionCheckoutBase || operationLabel(actionCheckoutBase) != "checking out base…" {
		t.Fatalf("checkout dispatch: %#v", m)
	}
	m, cmd = m.execute(actionDiscard)
	if cmd != nil || !m.confirm || m.pending != actionDiscard || operationLabel(actionDiscard) != "discarding changes…" {
		t.Fatalf("discard dispatch: %#v", m)
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil || updated.(Model).busy != actionDiscard {
		t.Fatalf("discard confirmation: %#v", updated)
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

func TestBranchInputAcceptsPaste(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m, _ = m.execute(actionAdd)
	updated, _ := m.Update(tea.PasteMsg{Content: "AG-123"})
	if got := updated.(Model).branch; got != "AG-123" {
		t.Fatalf("branch: %q", got)
	}
}

func TestOperationSpinnerAdvancesUntilResult(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m, _ = m.start(actionAdd)
	if !strings.Contains(m.View().Content, "| adding worktrees…") {
		t.Fatalf("missing spinner: %q", m.View().Content)
	}
	updated, cmd := m.Update(spinnerTick{})
	m = updated.(Model)
	if m.spinner != 1 || cmd == nil {
		t.Fatalf("spinner did not advance: %#v", m)
	}
	updated, _ = m.Update(operationResult{message: "done"})
	if updated.(Model).busy != "" {
		t.Fatalf("spinner did not stop: %#v", updated)
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

func TestLoadListsAndSortsWorktrees(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	web := tuiTestRepo(t, parent, "web")
	if _, err := worktree.Add(api, "AG-2", "main", config.Config{Layout: "sibling"}); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add(web, "AG-1", "main", config.Config{Layout: "sibling"}); err != nil {
		t.Fatal(err)
	}
	m := New(api, config.Config{})
	if m.Init() == nil {
		t.Fatal("init did not load")
	}
	fast := m.load(false)().(loaded)
	detailed := m.load(true)().(loaded)
	if fast.err != nil || detailed.err != nil || len(fast.items) != 4 || len(detailed.items) != 4 {
		t.Fatalf("load: fast=%#v detailed=%#v", fast, detailed)
	}
	for i := 1; i < len(fast.items); i++ {
		if fast.items[i-1].Branch > fast.items[i].Branch {
			t.Fatalf("items not sorted: %#v", fast.items)
		}
	}
}

func TestLoadReportsRepositoryError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	m := New(path, config.Config{})
	got := m.load(false)().(loaded)
	if got.err == nil || got.detailed {
		t.Fatalf("load error: %#v", got)
	}
}

func TestInputAndConfirmationCancellation(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m, _ = m.execute(actionAdd)
	m = press(m, "A")
	m = press(m, "backspace")
	m = press(m, "esc")
	if m.input || m.branch != "" {
		t.Fatalf("input was not cancelled: %#v", m)
	}
	m.feature = "AG-1"
	m.selected["/api.AG-1"] = true
	m, _ = m.execute(actionRemove)
	m = press(m, "n")
	if m.confirm {
		t.Fatalf("confirmation was not cancelled: %#v", m)
	}
}

func TestPaletteAndNavigationBounds(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: "/api", Primary: true}})
	m = press(m, "up")
	m = press(m, "down")
	m = press(m, "down")
	if m.cursor != 0 {
		t.Fatalf("cursor out of bounds: %d", m.cursor)
	}
	m = press(m, "space")
	m = press(m, "enter")
	for range 4 {
		m = press(m, "down")
	}
	if m.pCursor != len(m.availableActions())-1 {
		t.Fatalf("palette cursor: %d", m.pCursor)
	}
	for range 4 {
		m = press(m, "up")
	}
	if m.pCursor != 0 {
		t.Fatalf("palette cursor: %d", m.pCursor)
	}
}

func TestSelectedRepoPathsAndCommandHelpers(t *testing.T) {
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "main", Path: "/api", Primary: true},
		{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"},
		{Repo: "web", Branch: "main", Path: "/web", Primary: true},
		{Repo: "web", Branch: "AG-1", Path: "/web.AG-1"},
	})
	m.selected["/api"] = true
	if got := m.selectedRepoPaths(); !slices.Equal(got, []string{"/api"}) {
		t.Fatalf("roots: %v", got)
	}
	m.clearRoots()
	m.feature = "AG-1"
	m.selected["/api.AG-1"] = true
	m.selected["/web.AG-1"] = true
	if got := m.selectedRepoPaths(); !slices.Equal(got, []string{"/api", "/web"}) {
		t.Fatalf("features: %v", got)
	}
	if err := runAt("", t.TempDir()); err == nil {
		t.Fatal("empty command accepted")
	}
	if err := runAt("true", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "true")
	if err := openShell(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestPruneAndUpdateCommandsReload(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: api, Primary: true}})
	m.selected[api] = true
	result := m.pruneSelected()().(operationResult)
	if result.err != nil || !result.reload {
		t.Fatalf("prune result: %#v", result)
	}
	result = m.updateSelectedRoots()().(operationResult)
	if result.err == nil || !result.reload || !strings.Contains(result.err.Error(), "result may be partial") {
		t.Fatalf("update result: %#v", result)
	}
}

func TestOpenSelectedRunsConfiguredCommandAndRejectsEmptySelection(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: t.TempDir()}})
	m.feature = "AG-1"
	m.selected[m.items[0].Path] = true
	m.config.Editor = "true"
	result := m.openSelected(actionOpenEditor)().(operationResult)
	if result.err != nil || !result.reload {
		t.Fatalf("open result: %#v", result)
	}
	t.Setenv("SHELL", "true")
	result = m.openSelected(actionOpen)().(operationResult)
	if result.err != nil || !result.reload {
		t.Fatalf("shell result: %#v", result)
	}
	m.config.Agent = "true"
	result = m.openSelected(actionOpenAgent)().(operationResult)
	if result.err != nil || !result.reload {
		t.Fatalf("agent result: %#v", result)
	}
	m.clearSelection()
	result = m.openSelected(actionOpen)().(operationResult)
	if result.err == nil || !result.reload {
		t.Fatalf("empty open result: %#v", result)
	}
}

func TestSelectionOperationsReportRealBoundaryErrors(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: t.TempDir()}})
	m.feature = "AG-1"
	m.selected[m.items[0].Path] = true
	result := m.removeSelected()().(operationResult)
	if result.err == nil || !result.reload || !strings.Contains(result.err.Error(), "root for api not found") {
		t.Fatalf("remove result: %#v", result)
	}
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	m = modelWith([]worktree.Item{{Repo: "api", Branch: "main", Path: api, Primary: true}})
	m.selected[m.items[0].Path] = true
	result = m.addSelected()().(operationResult)
	if result.err == nil || !result.reload || !strings.Contains(result.err.Error(), "branch is required") {
		t.Fatalf("add result: %#v", result)
	}
}

func TestExecuteAndSelectionHelpersCoverContextualActions(t *testing.T) {
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "trunk", Path: "/api", Primary: true},
		{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"},
	})
	m.config.BaseBranch = "trunk"
	if _, ok := m.rootFor(worktree.Item{Repo: "web"}); m.baseBranch() != "trunk" || ok {
		t.Fatal("selection helpers")
	}
	m.feature = "AG-1"
	m.selected["/api.AG-1"] = true
	if _, ok := m.rootFor(m.items[1]); !ok {
		t.Fatal("feature root not found")
	}
	m, cmd := m.execute(actionPrune)
	if cmd == nil || m.palette {
		t.Fatal("prune was not dispatched")
	}
	m, cmd = m.execute(actionUpdate)
	if cmd == nil {
		t.Fatal("update was not dispatched")
	}
	m, cmd = m.execute(actionOpenAgent)
	if cmd == nil {
		t.Fatal("open was not dispatched")
	}
}

func TestViewHandlesEmptyAndColoredStates(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	os.Unsetenv("TERM")
	m := modelWith(nil)
	if !strings.Contains(m.View().Content, "(no worktrees)") {
		t.Fatal("empty state missing")
	}
	if got := plural(1); got != "" {
		t.Fatalf("singular suffix: %q", got)
	}
	if got := highlight("row"); !strings.Contains(got, "row") || got == "row" {
		t.Fatalf("highlight: %q", got)
	}
	if got := style("1", "text"); !strings.Contains(got, "text") || got == "text" {
		t.Fatalf("style: %q", got)
	}
}

func TestViewShowsInputConfirmationAndPalette(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := modelWith([]worktree.Item{{Repo: "api", Branch: "AG-1", Path: "/api.AG-1"}})
	m.feature = "AG-1"
	m.selected["/api.AG-1"] = true
	m.input, m.branch, m.confirm, m.palette, m.message = true, "AG-2", true, true, "branch: "
	view := m.View().Content
	for _, want := range []string{"branch: AG-2", "remove selected worktrees", "commands", "open"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q: %q", want, view)
		}
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
	m = updated.(Model)
	m.Update(m.addSelected()())
	if _, err := os.Stat(filepath.Join(parent, "api.AG-1")); err != nil {
		t.Fatalf("selected root not added: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "web.AG-1")); !os.IsNotExist(err) {
		t.Fatalf("unselected root changed: %v", err)
	}
}

func TestTUIAddRejectsExistingWorktreeBeforeCreatingAny(t *testing.T) {
	parent := t.TempDir()
	api := tuiTestRepo(t, parent, "api")
	web := tuiTestRepo(t, parent, "web")
	if _, err := worktree.Add(api, "AG-1", "main", config.Config{Layout: "sibling"}); err != nil {
		t.Fatal(err)
	}
	m := modelWith([]worktree.Item{
		{Repo: "api", Branch: "main", Path: api, Primary: true},
		{Repo: "web", Branch: "main", Path: web, Primary: true},
	})
	m.selected[api], m.selected[web], m.branch = true, true, "AG-1"
	result := m.addSelected()().(operationResult)
	if result.err == nil || !strings.Contains(result.err.Error(), "already exists in api") {
		t.Fatalf("result: %#v", result)
	}
	if strings.Contains(result.err.Error(), "result may be partial") {
		t.Fatalf("unexpected partial result: %v", result.err)
	}
	if _, err := os.Stat(filepath.Join(parent, "web.AG-1")); !os.IsNotExist(err) {
		t.Fatalf("web changed: %v", err)
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
	m.Update(m.removeSelected()())
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
