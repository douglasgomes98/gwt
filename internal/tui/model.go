package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

type Model struct {
	cwd       string
	config    config.Config
	items     []worktree.Item
	cursor    int
	confirm   bool
	input     bool
	selecting bool
	branch    string
	repos     []string
	selected  map[string]bool
	message   string
	detailed  bool
}
type loaded struct {
	items    []worktree.Item
	err      error
	detailed bool
}
type operationResult struct{ err error }

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
		m.detailed = x.detailed
		m.message = fmt.Sprintf("%d worktrees", len(m.items))
		if !m.detailed {
			m.message += " (checking status…)"
		}
		return m, nil
	case operationResult:
		if x.err != nil {
			m.message = x.err.Error()
		}
		return m, nil
	case tea.KeyPressMsg:
		if m.input {
			return m.typeBranch(x)
		}
		if m.selecting {
			return m.pickRepos(x)
		}
		if m.confirm {
			if x.String() == "y" && len(m.items) > 0 {
				branch := m.items[m.cursor].Branch
				for _, item := range m.items {
					if item.Branch == branch {
						if err := worktree.Remove(repoFor(m.cwd, item.Repo), branch); err != nil {
							m.message = err.Error()
							break
						}
					}
				}
				if m.message == "remove all worktrees for this branch? y/N" {
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
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "d":
			if len(m.items) > 0 {
				m.confirm = true
				m.message = "remove all worktrees for this branch? y/N"
			}
		case "p":
			repos, _ := worktree.Repos(m.cwd)
			for _, repo := range repos {
				_ = worktree.Prune(repo)
			}
			return m, m.reload()
		case "n":
			m.input = true
			m.branch = ""
			m.message = "branch: "
		case "enter":
			if len(m.items) > 0 {
				return m, openShell(m.items[m.cursor].Path)
			}
		case "e":
			if len(m.items) > 0 {
				return m, run(m.config.Editor, m.items[m.cursor].Path)
			}
		case "a":
			if len(m.items) > 0 {
				return m, run(m.config.Agent, m.items[m.cursor].Path)
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
		repos, err := worktree.Repos(m.cwd)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.repos, m.selected, m.selecting = repos, map[string]bool{}, true
		m.cursor = 0
		for _, repo := range repos {
			m.selected[repo] = true
		}
		m.message = "space: toggle  enter: create  esc: cancel"
		return m, nil
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

func (m Model) pickRepos(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.selecting = false
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.repos)-1 {
			m.cursor++
		}
	case " ":
		if len(m.repos) > 0 {
			repo := m.repos[m.cursor]
			m.selected[repo] = !m.selected[repo]
		}
	case "enter":
		for _, repo := range m.repos {
			if !m.selected[repo] {
				continue
			}
			_ = worktree.Fetch(repo, m.config.BaseBranch)
			if _, err := worktree.Add(repo, m.branch, "origin/"+m.config.BaseBranch, m.config); err != nil {
				m.message = err.Error()
				m.selecting = false
				return m, nil
			}
		}
		m.selecting = false
		m.cursor = 0
		m.message = "created " + m.branch
		return m, m.reload()
	}
	return m, nil
}
func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString("gwt  n:new  d:remove  enter:shell  e:editor  a:agent  p:prune  q:quit\n\n")
	last := ""
	for i, item := range m.items {
		if item.Branch != last {
			last = item.Branch
			b.WriteString("[" + last + "]\n")
		}
		mark := " "
		if i == m.cursor {
			mark = ">"
		}
		dirty := ""
		if item.Dirty {
			dirty = " dirty"
		}
		fmt.Fprintf(&b, "%s %-18s %s%s +%d/-%d\n", mark, item.Repo, item.Path, dirty, item.Ahead, item.Behind)
	}
	if len(m.items) == 0 {
		b.WriteString("(no worktrees)\n")
	}
	b.WriteString("\n" + m.message)
	if m.input {
		b.WriteString(m.branch)
	}
	if m.selecting {
		b.WriteString("\nselect repositories:\n")
		for i, repo := range m.repos {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			checked := " "
			if m.selected[repo] {
				checked = "x"
			}
			fmt.Fprintf(&b, "%s [%s] %s\n", cursor, checked, repo)
		}
	}
	return tea.NewView(b.String())
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
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	return tea.ExecProcess(exec.Command(sh), func(err error) tea.Msg { return operationResult{err} })
}
func run(command, dir string) tea.Cmd {
	if command == "" {
		return func() tea.Msg { return operationResult{fmt.Errorf("command is not configured")} }
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return operationResult{err} })
}
