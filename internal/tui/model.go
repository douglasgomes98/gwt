package tui

import (
	"fmt"
	"os"
	"os/exec"
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
	confirm  bool
	input    bool
	branch   string
	projects map[string]bool
	group    string
	message  string
	detailed bool
}
type loaded struct {
	items    []worktree.Item
	err      error
	detailed bool
}
type operationResult struct {
	err     error
	message string
}

func New(cwd string, c config.Config) Model { return Model{cwd: cwd, config: c, message: "loading…"} }
func (m Model) Init() tea.Cmd               { return m.reload() }
func (m Model) reload() tea.Cmd             { return tea.Batch(m.load(false), m.load(true)) }
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
		if m.projects == nil {
			m.projects = map[string]bool{}
		}
		for _, item := range x.items {
			if m.isProject(item) {
				_, ok := m.projects[item.Path]
				if !ok {
					m.projects[item.Path] = false
				}
			}
		}
		m.detailed = x.detailed
		m.message = fmt.Sprintf("%d worktrees", len(m.items))
		if !m.detailed {
			m.message += " (checking status…)"
		}
		return m, nil
	case operationResult:
		if x.err != nil {
			m.message = x.err.Error()
		} else if x.message != "" {
			m.message = x.message
		}
		return m, nil
	case tea.KeyPressMsg:
		if m.input {
			return m.typeBranch(x)
		}
		if m.confirm {
			if (x.String() == "y" || x.String() == "enter") && len(m.items) > 0 {
				branch := m.group
				for _, item := range m.items {
					if item.Branch == branch {
						if err := worktree.Remove(repoFor(m.cwd, item.Repo), branch); err != nil {
							m.message = err.Error()
							break
						}
					}
				}
				if m.message == "remove all worktrees for this branch? y/Enter/N" {
					m.message = "removed " + branch
				}
				m.confirm = false
				return m, m.reload()
			}
			m.confirm = false
			return m, nil
		}
		switch x.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.group = ""
			for path := range m.projects {
				m.projects[path] = false
			}
			m.message = "selection cleared"
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "d":
			if m.group != "" && m.canRemoveGroup() {
				m.confirm = true
				m.message = "remove all worktrees for this branch? y/Enter/N"
			}
		case "p":
			repos, _ := worktree.Repos(m.cwd)
			for _, repo := range repos {
				_ = worktree.Prune(repo)
			}
			return m, m.reload()
		case "u":
			if m.group != "" {
				return m, m.updateGroup()
			}
		case "n":
			if m.projectCount() == 0 {
				m.message = "select primary projects with space first"
			} else {
				m.input = true
				m.branch = ""
				m.message = "branch: "
			}
		case " ", "space":
			if len(m.items) > 0 {
				item := m.items[m.cursor]
				if m.isProject(item) {
					m.projects[item.Path] = !m.projects[item.Path]
				} else if m.group != "" && m.group != item.Branch {
					m.message = "press esc to change worktree group"
				} else if m.group == item.Branch {
					m.group = ""
				} else {
					m.group = item.Branch
				}
			}
		case "enter":
			if item, ok := m.activeItem(); ok {
				return m, openShell(item.Path)
			}
		case "e":
			if item, ok := m.activeItem(); ok {
				return m, run(m.config.Editor, item.Path)
			}
		case "a":
			if item, ok := m.activeItem(); ok {
				return m, run(m.config.Agent, item.Path)
			}
		}
	}
	return m, nil
}
func (m Model) typeBranch(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch s {
	case "esc":
		m.input = false
		return m, nil
	case "enter":
		m.input = false
		if m.branch == "" {
			return m, nil
		}
		for _, item := range m.items {
			if !m.isProject(item) || !m.projects[item.Path] {
				continue
			}
			_ = worktree.Fetch(item.Path, m.config.BaseBranch)
			if _, err := worktree.Add(item.Path, m.branch, "origin/"+m.config.BaseBranch, m.config); err != nil {
				m.message = err.Error()
				return m, nil
			}
		}
		m.message = "created " + m.branch
		return m, m.reload()
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

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(style("1;38;5;81", "gwt") + "\n\n")
	last := ""
	branch := m.selectedBranch()
	for i, item := range m.items {
		if item.Branch != last {
			last = item.Branch
			header := last + "  " + style("2", fmt.Sprintf("%d worktrees", m.groupSize(last)))
			if last == branch {
				header = style("1;38;5;141", last) + "  " + style("1;38;5;114", fmt.Sprintf("%s selected", worktreeCount(m.groupSize(last))))
			}
			b.WriteString(header + "\n")
		}
		mark := " "
		radio := style("2", "○")
		selected := (!m.isProject(item) && item.Branch == branch) || (m.isProject(item) && m.projects[item.Path])
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
	if m.projectCount() > 0 {
		b.WriteString("\n" + style("1;38;5;114", fmt.Sprintf("%s selected for new branch", projectCount(m.projectCount()))) + "  " + style("1", "space") + " toggle  " + style("1", "n") + " new branch")
	}
	if branch != "" {
		b.WriteString("\n" + style("1", "Enter") + " shell  " + style("1", "esc") + " cancel  " + style("1", "e") + " editor  " + style("1", "a") + " agent")
		if m.canRemoveGroup() {
			b.WriteString("  " + style("1;38;5;208", "d") + " remove group")
		}
		b.WriteString("  " + style("1", "p") + " prune  " + style("1", "u") + " update " + m.baseBranch() + "  " + style("1", "q") + " quit")
	}
	if m.projectCount() > 0 && branch == "" {
		b.WriteString("  " + style("1", "esc") + " cancel  " + style("1", "p") + " prune  " + style("1", "q") + " quit")
	}
	if m.input {
		b.WriteString(m.branch)
	}
	return tea.NewView(b.String())
}

func displayPath(path string) string { return filepath.Base(path) }

func worktreeCount(n int) string { return fmt.Sprintf("%d worktree", n) + plural(n) }

func projectCount(n int) string { return fmt.Sprintf("%d project", n) + plural(n) }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) selectedBranch() string {
	return m.group
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

func (m Model) canRemoveGroup() bool {
	for _, item := range m.items {
		if item.Branch == m.group && item.Primary {
			return false
		}
	}
	return true
}

func (m Model) updateGroup() tea.Cmd {
	branch, base, items := m.group, m.baseBranch(), m.items
	return func() tea.Msg {
		for _, item := range items {
			if item.Branch == branch {
				if err := worktree.Update(item.Path, base); err != nil {
					return operationResult{err: err}
				}
			}
		}
		return operationResult{message: "updated from " + base}
	}
}

func (m Model) activeItem() (worktree.Item, bool) {
	if m.group == "" {
		return worktree.Item{}, false
	}
	if m.cursor >= 0 && m.cursor < len(m.items) && m.items[m.cursor].Branch == m.group {
		return m.items[m.cursor], true
	}
	for _, item := range m.items {
		if item.Branch == m.group {
			return item, true
		}
	}
	return worktree.Item{}, false
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

func (m Model) projectCount() int {
	n := 0
	for _, selected := range m.projects {
		if selected {
			n++
		}
	}
	return n
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
func repoFor(cwd, name string) string {
	repos, _ := worktree.Repos(cwd)
	for _, r := range repos {
		if strings.TrimSuffix(r, "/") != "" && strings.HasSuffix(r, "/"+name) {
			return r
		}
	}
	return cwd
}
func openShell(dir string) tea.Cmd {
	return tea.ExecProcess(shellCommand(dir), func(err error) tea.Msg { return operationResult{err: err} })
}

func shellCommand(dir string) *exec.Cmd {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	cmd := exec.Command(sh)
	cmd.Dir = dir
	return cmd
}
func run(command, dir string) tea.Cmd {
	if command == "" {
		return func() tea.Msg { return operationResult{err: fmt.Errorf("command is not configured")} }
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return operationResult{err: err} })
}
