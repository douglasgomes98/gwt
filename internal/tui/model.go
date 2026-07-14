package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	message  string
	detailed bool
}

type action string

const (
	actionAdd        action = "add"
	actionAddAll     action = "add --all"
	actionOpen       action = "open"
	actionOpenEditor action = "open -e"
	actionOpenAgent  action = "open -a"
	actionRemove     action = "rm"
	actionRemoveAll  action = "rm --all"
	actionPrune      action = "prune"
	actionUpdate     action = "update"
)

type loaded struct {
	items    []worktree.Item
	err      error
	detailed bool
}

func New(cwd string, c config.Config) Model {
	return Model{cwd: cwd, config: c, selected: map[string]bool{}, message: "loading…"}
}
func (m Model) Init() tea.Cmd   { return m.reload() }
func (m Model) reload() tea.Cmd { return tea.Batch(m.load(false), m.load(true)) }
func (m Model) load(detailed bool) tea.Cmd {
	return func() tea.Msg {
		repos, err := worktree.Repos(m.cwd)
		if err != nil {
			return loaded{err: err, detailed: detailed}
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
				return loaded{err: err, detailed: detailed}
			}
			items = append(items, xs...)
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Branch == items[j].Branch {
				return items[i].Repo < items[j].Repo
			}
			return items[i].Branch < items[j].Branch
		})
		return loaded{items: items, detailed: detailed}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case loaded:
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
	case tea.KeyPressMsg:
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
				m.palette = false
			}
			return m, nil
		}
		switch x.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
					if m.feature != item.Branch {
						m.feature = item.Branch
						m.clearRoots()
						for _, candidate := range m.items {
							if candidate.Branch == item.Branch && !candidate.Detached {
								m.selected[candidate.Path] = true
							}
						}
					} else {
						m.selected[item.Path] = !m.selected[item.Path]
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

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(style("1;38;5;81", "gwt") + "\n\n")
	last := ""
	for i, item := range m.items {
		if item.Branch != last {
			last = item.Branch
			header := last + "  " + style("2", fmt.Sprintf("%d worktrees", m.groupSize(last)))
			if last == m.feature {
				header = style("1;38;5;141", last) + "  " + style("1;38;5;114", fmt.Sprintf("%s selected", worktreeCount(len(m.selectedFeatureItems()))))
			}
			b.WriteString(header + "\n")
		}
		mark := " "
		radio := style("2", "○")
		selected := m.selected[item.Path]
		if selected {
			radio = style("1;38;5;114", "◉")
		}
		if i == m.cursor {
			mark = "›"
		}
		repo := style(repoColor(item.Repo), fmt.Sprintf("%-18s", item.Repo))
		path := style("2", fmt.Sprintf("%-42s", displayPath(item.Path)))
		row := fmt.Sprintf("%s %s %s %s %s", mark, radio, repo, path, itemStatus(item))
		if selected {
			row = highlight(row)
		}
		b.WriteString(row + "\n")
	}
	if len(m.items) == 0 {
		b.WriteString(style("2", "(no worktrees)") + "\n")
	}
	b.WriteString("\n" + style("2", m.message))
	if m.palette {
		b.WriteString("\n\n" + style("1", "commands") + "\n")
		for i, action := range m.availableActions() {
			mark := " "
			if i == m.pCursor {
				mark = "›"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", mark, action))
		}
		b.WriteString(style("2", "enter select  esc close"))
	} else if len(m.availableActions()) > 0 {
		b.WriteString("\n" + style("1", "Enter") + " commands  " + style("1", "q") + " quit")
	}
	return tea.NewView(b.String())
}

func displayPath(path string) string { return filepath.Base(path) }

func worktreeCount(n int) string { return fmt.Sprintf("%d worktree", n) + plural(n) }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) isProject(item worktree.Item) bool {
	return item.Primary && item.Branch == m.baseBranch()
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
		if item.Branch == m.feature {
			m.selected[item.Path] = false
		}
	}
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
		if item.Branch == m.feature && !item.Detached && m.selected[item.Path] {
			items = append(items, item)
		}
	}
	return items
}

func (m Model) groupSize(branch string) int {
	n := 0
	for _, item := range m.items {
		if item.Branch == branch {
			n++
		}
	}
	return n
}

func (m Model) availableActions() []action {
	if roots := m.selectedRoots(); len(roots) > 0 {
		if len(roots) == 1 {
			return []action{actionAdd, actionPrune, actionUpdate}
		}
		return []action{actionAddAll, actionPrune, actionUpdate}
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

func repoColor(repo string) string {
	palette := [...]string{"38;5;75", "38;5;81", "38;5;114", "38;5;141", "38;5;215"}
	n := 0
	for _, r := range repo {
		n += int(r)
	}
	return palette[n%len(palette)]
}
