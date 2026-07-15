package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

type Model struct {
	cwd      string
	config   config.Config
	items    []worktree.Item
	cursor   int
	selected map[string]bool
	feature  string
	palette  bool
	pCursor  int
	input    bool
	branch   string
	confirm  bool
	pending  action
	message  string
	result   string
	busy     action
	spinner  int
	detailed bool
	loadID   int
}

type action string

const (
	actionAdd          action = "add"
	actionAddAll       action = "add --all"
	actionOpen         action = "open"
	actionOpenEditor   action = "open -e"
	actionOpenAgent    action = "open -a"
	actionRemove       action = "rm"
	actionRemoveAll    action = "rm --all"
	actionPrune        action = "prune"
	actionUpdate       action = "update"
	actionCheckoutBase action = "checkout-base"
	actionDiscard      action = "discard"
)

type loaded struct {
	items    []worktree.Item
	err      error
	detailed bool
	loadID   int
}

type operationResult struct {
	err     error
	message string
	reload  bool
}

type spinnerTick struct{}

var spinnerFrames = []string{"|", "/", "-", "\\"}

func New(cwd string, c config.Config) Model {
	return Model{cwd: cwd, config: c, selected: map[string]bool{}, message: "loading…", loadID: 1}
}
func (m Model) Init() tea.Cmd   { return m.reload() }
func (m Model) reload() tea.Cmd { return tea.Batch(m.load(false), m.load(true)) }
func (m Model) load(detailed bool) tea.Cmd {
	return func() tea.Msg {
		repos, err := worktree.Repos(m.cwd)
		if err != nil {
			return loaded{err: err, detailed: detailed, loadID: m.loadID}
		}
		var items []worktree.Item
		for _, repo := range repos {
			var xs []worktree.Item
			if detailed {
				xs, err = worktree.List(repo)
			} else {
				xs, err = worktree.ListFast(repo)
			}
			if err != nil {
				return loaded{err: err, detailed: detailed, loadID: m.loadID}
			}
			items = append(items, xs...)
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Primary != items[j].Primary {
				return !items[i].Primary
			}
			if items[i].Branch == items[j].Branch {
				return items[i].Repo < items[j].Repo
			}
			return items[i].Branch < items[j].Branch
		})
		return loaded{items: items, detailed: detailed, loadID: m.loadID}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case operationResult:
		m.palette, m.confirm, m.input = false, false, false
		m.pending = ""
		m.busy, m.spinner = "", 0
		m.clearSelection()
		if x.err != nil {
			m.result = x.err.Error()
		} else {
			m.result = x.message
		}
		if x.reload {
			m.loadID++
			return m, m.reload()
		}
		return m, nil
	case spinnerTick:
		if m.busy == "" {
			return m, nil
		}
		m.spinner = (m.spinner + 1) % len(spinnerFrames)
		return m, m.nextSpinner()
	case loaded:
		if x.loadID != m.loadID {
			return m, nil
		}
		if m.detailed && !x.detailed {
			return m, nil
		}
		if x.err != nil {
			if !x.detailed || !m.detailed {
				m.message = x.err.Error()
			}
			return m, nil
		}
		m.items = x.items
		m.detailed = x.detailed
		m.message = fmt.Sprintf("%d worktrees", len(m.items))
		if !m.detailed {
			m.message += " (checking status…)"
		}
		return m, nil
	case tea.PasteMsg:
		if m.input {
			m.branch += x.Content
		}
		return m, nil
	case tea.KeyPressMsg:
		m.result = ""
		if m.confirm {
			switch x.String() {
			case "esc", "n":
				m.confirm = false
			case "enter", "y":
				m.confirm = false
				m, tick := m.start(m.pending)
				if m.pending == actionDiscard {
					return m, tea.Batch(tick, m.discardSelectedRoots())
				}
				return m, tea.Batch(tick, m.removeSelected())
			}
			return m, nil
		}
		if m.input {
			return m.typeBranch(x)
		}
		if m.palette {
			switch x.String() {
			case "esc":
				m.palette = false
			case "down", "j":
				if m.pCursor < len(m.availableActions())-1 {
					m.pCursor++
				}
			case "up", "k":
				if m.pCursor > 0 {
					m.pCursor--
				}
			case "enter":
				if actions := m.availableActions(); m.pCursor < len(actions) {
					return m.execute(actions[m.pCursor])
				}
			}
			return m, nil
		}
		switch x.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.clearSelection()
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ", "space":
			if len(m.items) > 0 {
				item := m.items[m.cursor]
				if m.isProject(item) {
					m.clearFeature()
					m.selected[item.Path] = !m.selected[item.Path]
				} else if !item.Detached {
					if m.feature == "" {
						m.feature = item.Branch
						m.clearRoots()
						for _, candidate := range m.items {
							if candidate.Branch == item.Branch && !candidate.Detached {
								m.selected[candidate.Path] = true
							}
						}
					} else if m.feature == item.Branch {
						m.selected[item.Path] = !m.selected[item.Path]
						if len(m.selectedFeatureItems()) == 0 {
							m.clearFeature()
						}
					}
				}
			}
		case "enter":
			if len(m.availableActions()) > 0 {
				m.palette = true
				m.pCursor = 0
			}
		}
	}
	return m, nil
}

func (m Model) typeBranch(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch s := k.String(); s {
	case "esc":
		m.input = false
	case "enter":
		m.input = false
		m, tick := m.start(actionAdd)
		return m, tea.Batch(tick, m.addSelected())
	case "backspace":
		if len(m.branch) > 0 {
			m.branch = m.branch[:len(m.branch)-1]
		}
	default:
		if len(s) == 1 {
			m.branch += s
		}
	}
	return m, nil
}

func (m Model) execute(a action) (Model, tea.Cmd) {
	switch a {
	case actionAdd, actionAddAll:
		m.palette, m.input, m.branch = false, true, ""
		m.message = "branch: "
		return m, nil
	case actionRemove, actionRemoveAll, actionDiscard:
		m.palette, m.confirm, m.pending = false, true, a
		return m, nil
	case actionPrune:
		m, tick := m.start(a)
		return m, tea.Batch(tick, m.pruneSelected())
	case actionUpdate:
		m, tick := m.start(a)
		return m, tea.Batch(tick, m.updateSelectedRoots())
	case actionCheckoutBase:
		m, tick := m.start(a)
		return m, tea.Batch(tick, m.checkoutBaseSelectedRoots())
	default:
		return m, m.openSelected(a)
	}
}

func (m Model) start(a action) (Model, tea.Cmd) {
	m.busy, m.spinner = a, 0
	return m, m.nextSpinner()
}

func (m Model) nextSpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return spinnerTick{} })
}

func (m Model) addSelected() tea.Cmd {
	roots, branch := m.selectedRoots(), m.branch
	return func() tea.Msg {
		for _, root := range roots {
			items, err := worktree.ListFast(root.Path)
			if err != nil {
				return operationResult{err: fmt.Errorf("add: inspect %s: %w", root.Repo, err), reload: true}
			}
			for _, item := range items {
				if item.Branch == branch {
					return operationResult{err: fmt.Errorf("add: branch %q already exists in %s", branch, root.Repo), reload: true}
				}
			}
		}
		for _, root := range roots {
			if _, err := worktree.Add(root.Path, branch, m.baseBranch(), m.config); err != nil {
				return operationResult{err: partial(actionAdd, err), reload: true}
			}
		}
		return operationResult{message: fmt.Sprintf("added %d worktrees", len(roots)), reload: true}
	}
}

func (m Model) removeSelected() tea.Cmd {
	items := m.selectedFeatureItems()
	return func() tea.Msg {
		for _, item := range items {
			root, ok := m.rootFor(item)
			if !ok {
				return operationResult{err: partial(m.pending, fmt.Errorf("root for %s not found", item.Repo)), reload: true}
			}
			if err := worktree.Remove(root.Path, item.Branch); err != nil {
				return operationResult{err: partial(m.pending, err), reload: true}
			}
		}
		return operationResult{message: fmt.Sprintf("removed %d worktrees", len(items)), reload: true}
	}
}

func (m Model) pruneSelected() tea.Cmd {
	repos := m.selectedRepoPaths()
	return func() tea.Msg {
		for _, repo := range repos {
			if err := worktree.Prune(repo); err != nil {
				return operationResult{err: partial(actionPrune, err), reload: true}
			}
		}
		return operationResult{message: fmt.Sprintf("pruned %d repos", len(repos)), reload: true}
	}
}

func (m Model) updateSelectedRoots() tea.Cmd {
	roots := m.selectedRoots()
	return func() tea.Msg {
		for _, root := range roots {
			if err := worktree.Update(root.Path, m.baseBranch()); err != nil {
				return operationResult{err: partial(actionUpdate, err), reload: true}
			}
		}
		return operationResult{message: fmt.Sprintf("updated %d roots", len(roots)), reload: true}
	}
}

func (m Model) checkoutBaseSelectedRoots() tea.Cmd {
	roots := m.selectedRoots()
	return func() tea.Msg {
		for _, root := range roots {
			if err := worktree.CheckoutBase(root.Path, m.baseBranch()); err != nil {
				return operationResult{err: partial(actionCheckoutBase, err), reload: true}
			}
		}
		return operationResult{message: fmt.Sprintf("checked out base in %d roots", len(roots)), reload: true}
	}
}

func (m Model) discardSelectedRoots() tea.Cmd {
	roots := m.selectedRoots()
	return func() tea.Msg {
		for _, root := range roots {
			if err := worktree.Discard(root.Path); err != nil {
				return operationResult{err: partial(actionDiscard, err), reload: true}
			}
		}
		return operationResult{message: fmt.Sprintf("discarded changes in %d roots", len(roots)), reload: true}
	}
}

func (m Model) openSelected(a action) tea.Cmd {
	items := m.selectedFeatureItems()
	return func() tea.Msg {
		if len(items) != 1 {
			return operationResult{err: fmt.Errorf("select one worktree to open"), reload: true}
		}
		var err error
		switch a {
		case actionOpenEditor:
			err = runAt(m.config.Editor, items[0].Path)
		case actionOpenAgent:
			err = runAt(m.config.Agent, items[0].Path)
		default:
			err = openShell(items[0].Path)
		}
		return operationResult{err: err, message: "opened " + items[0].Path, reload: true}
	}
}

func (m Model) rootFor(item worktree.Item) (worktree.Item, bool) {
	for _, root := range m.items {
		if root.Repo == item.Repo && m.isProject(root) {
			return root, true
		}
	}
	return worktree.Item{}, false
}

func (m Model) selectedRepoPaths() []string {
	seen := map[string]bool{}
	var repos []string
	for _, root := range m.selectedRoots() {
		if !seen[root.Path] {
			seen[root.Path] = true
			repos = append(repos, root.Path)
		}
	}
	for _, item := range m.selectedFeatureItems() {
		if root, ok := m.rootFor(item); ok && !seen[root.Path] {
			seen[root.Path] = true
			repos = append(repos, root.Path)
		}
	}
	return repos
}

func partial(a action, err error) error { return fmt.Errorf("%s: result may be partial: %w", a, err) }

func runAt(command, dir string) error {
	if command == "" {
		return fmt.Errorf("command is not configured")
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func openShell(dir string) error {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	return runAt(sh, dir)
}

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(style("1;38;5;81", "gwt") + "\n\n")
	groups := map[string][]int{}
	var branches []string
	var roots []int
	for i, item := range m.items {
		if item.Primary {
			roots = append(roots, i)
			continue
		}
		branch := item.Branch
		if item.Detached {
			branch = "(detached)"
		}
		if _, ok := groups[branch]; !ok {
			branches = append(branches, branch)
		}
		groups[branch] = append(groups[branch], i)
	}
	renderRows := func(rows []int) {
		for _, i := range rows {
			item := m.items[i]
			mark := " "
			radio := style("2", "○")
			selected := m.selected[item.Path]
			if selected {
				radio = style("1;38;5;114", "◉")
			}
			if i == m.cursor {
				mark = "›"
			}
			nameStyle := "1"
			if !item.Primary {
				nameStyle = "1;38;5;81"
			}
			repo := style(nameStyle, fmt.Sprintf("%-18s", item.Repo))
			path := style("2", fmt.Sprintf("%-42s", displayPath(item.Path)))
			row := fmt.Sprintf("%s %s %s %s %s", mark, radio, repo, path, itemStatus(item))
			if selected {
				row = highlight(row)
			}
			b.WriteString(row + "\n")
		}
	}
	for _, branch := range branches {
		header := branch + "  " + style("2", worktreeCount(len(groups[branch])))
		if m.feature != "" && branch == m.feature {
			header = style("1;38;5;141", branch) + "  " + style("1;38;5;114", fmt.Sprintf("%s selected", worktreeCount(len(m.selectedFeatureItems()))))
		}
		b.WriteString(header + "\n")
		renderRows(groups[branch])
	}
	if len(roots) > 0 {
		b.WriteString("roots  " + style("2", rootCount(len(roots))) + "\n")
		renderRows(roots)
	}
	if len(m.items) == 0 {
		b.WriteString(style("2", "(no worktrees)") + "\n")
	}
	status := m.message
	if m.result != "" {
		status = m.result
	}
	if m.busy != "" {
		status = fmt.Sprintf("%s %s", spinnerFrames[m.spinner], operationLabel(m.busy))
	}
	b.WriteString("\n" + style("2", status))
	if m.input {
		b.WriteString(m.branch)
		b.WriteString("  " + keyHint("enter", "create", "1;38;5;114", "0") + "  " + keyHint("esc", "cancel", "2", "2"))
	}
	if m.confirm {
		prompt := "remove selected worktrees?"
		promptStyle := "1;38;5;208"
		if m.pending == actionDiscard {
			prompt = "discard all local changes in selected roots?"
			promptStyle = "1;38;5;203"
		}
		b.WriteString("\n" + style(promptStyle, prompt) + "  " + keyHint("enter/y", "confirm", "1;38;5;114", "0") + "  " + keyHint("esc/n", "cancel", "2", "2"))
	}
	if m.palette {
		b.WriteString("\n\n" + style("1", "commands") + "\n")
		for i, action := range m.availableActions() {
			mark := " "
			if i == m.pCursor {
				mark = "›"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", mark, actionLabel(action)))
		}
		b.WriteString(keyHint("enter", "select", "1;38;5;114", "0") + "  " + keyHint("esc", "close", "2", "2"))
	} else if !m.input && !m.confirm && len(m.availableActions()) > 0 {
		b.WriteString("\n" + keyHint("enter", "commands", "1;38;5;114", "0") + "  " + keyHint("q", "quit", "2", "2"))
	}
	return tea.NewView(b.String())
}

func displayPath(path string) string { return filepath.Base(path) }

func keyHint(keys, label, keyStyle, labelStyle string) string {
	return style(keyStyle, "["+keys+"]") + " " + style(labelStyle, label)
}

func actionLabel(a action) string {
	switch a {
	case actionAdd:
		return "create worktree"
	case actionAddAll:
		return "create worktrees"
	case actionOpen:
		return "open shell"
	case actionOpenEditor:
		return "open editor"
	case actionOpenAgent:
		return "open agent"
	case actionRemove:
		return "remove worktree"
	case actionRemoveAll:
		return "remove worktrees"
	case actionPrune:
		return "prune stale worktrees"
	case actionUpdate:
		return "update root"
	case actionCheckoutBase:
		return "checkout base branch"
	case actionDiscard:
		return "discard local changes"
	}
	return string(a)
}

func operationLabel(a action) string {
	switch a {
	case actionAdd, actionAddAll:
		return "adding worktrees…"
	case actionRemove, actionRemoveAll:
		return "removing worktrees…"
	case actionPrune:
		return "pruning worktrees…"
	case actionUpdate:
		return "updating roots…"
	case actionCheckoutBase:
		return "checking out base…"
	case actionDiscard:
		return "discarding changes…"
	}
	return "working…"
}

func worktreeCount(n int) string { return fmt.Sprintf("%d worktree", n) + plural(n) }
func rootCount(n int) string     { return fmt.Sprintf("%d root", n) + plural(n) }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) isProject(item worktree.Item) bool {
	return item.Primary && !item.Detached
}

func (m Model) baseBranch() string {
	if m.config.BaseBranch == "" {
		return "main"
	}
	return m.config.BaseBranch
}

func (m *Model) clearRoots() {
	for _, item := range m.items {
		if m.isProject(item) {
			m.selected[item.Path] = false
		}
	}
}

func (m *Model) clearFeature() {
	for _, item := range m.items {
		if !item.Primary && item.Branch == m.feature {
			m.selected[item.Path] = false
		}
	}
	m.feature = ""
}

func (m *Model) clearSelection() {
	m.selected = map[string]bool{}
	m.feature = ""
}

func (m Model) selectedRoots() []worktree.Item {
	var roots []worktree.Item
	for _, item := range m.items {
		if m.isProject(item) && m.selected[item.Path] {
			roots = append(roots, item)
		}
	}
	return roots
}

func (m Model) selectedFeatureItems() []worktree.Item {
	var items []worktree.Item
	for _, item := range m.items {
		if !item.Primary && item.Branch == m.feature && !item.Detached && m.selected[item.Path] {
			items = append(items, item)
		}
	}
	return items
}

func (m Model) availableActions() []action {
	if roots := m.selectedRoots(); len(roots) > 0 {
		add := actionAdd
		if len(roots) > 1 {
			add = actionAddAll
		}
		actions := []action{add, actionPrune}
		allClean, allOnBase, anyDirty := true, true, false
		for _, root := range roots {
			allClean = allClean && !root.Dirty
			allOnBase = allOnBase && root.Branch == m.baseBranch()
			anyDirty = anyDirty || root.Dirty
		}
		if allClean && allOnBase {
			actions = append(actions, actionUpdate)
		}
		if allClean {
			actions = append(actions, actionCheckoutBase)
		}
		if anyDirty {
			actions = append(actions, actionDiscard)
		}
		return actions
	}
	items := m.selectedFeatureItems()
	if len(items) == 0 {
		return nil
	}
	if len(items) > 1 {
		return []action{actionRemoveAll, actionPrune}
	}
	actions := []action{actionOpen}
	if m.config.Editor != "" {
		actions = append(actions, actionOpenEditor)
	}
	if m.config.Agent != "" {
		actions = append(actions, actionOpenAgent)
	}
	return append(actions, actionRemove, actionPrune)
}

func itemStatus(item worktree.Item) string {
	var parts []string
	if item.Changes > 0 {
		parts = append(parts, style("1;38;5;208", fmt.Sprintf("%d files changed", item.Changes)))
	}
	if item.Ahead > 0 {
		parts = append(parts, style("38;5;114", fmt.Sprintf("ahead %d", item.Ahead)))
	}
	if item.Behind > 0 {
		parts = append(parts, style("38;5;203", fmt.Sprintf("behind %d", item.Behind)))
	}
	if len(parts) == 0 {
		parts = append(parts, style("2", "clean"))
	}
	return "(" + strings.Join(parts, style("2", " · ")) + ")"
}

func style(code, text string) string {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func highlight(text string) string {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return text
	}
	const background = "\033[48;5;238m"
	return background + strings.ReplaceAll(text, "\033[0m", "\033[0m"+background) + "\033[0m"
}
