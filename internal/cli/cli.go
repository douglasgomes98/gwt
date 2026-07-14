package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

type App struct {
	Out, Err io.Writer
	Dir      string
	Version  string
	Config   config.Config
}

func New(out, err io.Writer, dir, version string, cfg config.Config) App {
	return App{Out: out, Err: err, Dir: dir, Version: version, Config: cfg}
}

func (a App) Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command")
	}
	switch args[0] {
	case "version":
		return a.version(args[1:])
	case "help":
		return a.help(args[1:])
	case "add":
		return a.add(args[1:])
	case "open":
		return a.open(args[1:])
	case "rm":
		return a.remove(args[1:])
	case "list":
		return a.list(args[1:])
	case "prune":
		return a.prune()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a App) version(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt version")
	}
	_, err := fmt.Fprintln(a.Out, a.Version)
	return err
}

func (a App) help(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt help")
	}
	_, err := fmt.Fprint(a.Out, `Usage: gwt <command>

Commands:
  add <branch> [base] [-e|-a] [--all]  Create a worktree.
  open <branch> [-e|-a|-p]             Open a worktree.
  rm <branch>                          Remove a worktree.
  list                                 List worktrees.
  prune                                Prune stale worktrees.
  version                              Show the version.
  help                                 Show this help.

Run gwt without a command to open the TUI.
`)
	return err
}

func (a App) add(args []string) error {
	args = flagsFirst(args)
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	all := fs.Bool("all", false, "add in all sibling repos")
	editor := fs.Bool("e", false, "open editor")
	agent := fs.Bool("a", false, "open agent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: gwt add <branch> [base] [-e|-a] [--all]")
	}
	base := a.Config.BaseBranch
	if len(rest) == 2 {
		base = rest[1]
	}
	repos := []string{}
	if *all {
		var err error
		repos, err = worktree.Repos(a.Dir)
		if err != nil {
			return err
		}
	} else {
		repo, err := worktree.CurrentRepo(a.Dir)
		if err != nil {
			return err
		}
		repos = []string{repo}
	}
	for _, repo := range repos {
		if *all {
			_ = worktree.Fetch(repo, base)
		}
		path, err := worktree.Add(repo, rest[0], base, a.Config)
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, path)
		if *editor {
			return runAt(a.Config.Editor, path)
		}
		if *agent {
			return runAt(a.Config.Agent, path)
		}
	}
	return nil
}
func (a App) open(args []string) error {
	args = flagsFirst(args)
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	editor := fs.Bool("e", false, "editor")
	agent := fs.Bool("a", false, "agent")
	pathOnly := fs.Bool("p", false, "print path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: gwt open <branch> [-e|-a|-p]")
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	items, err := worktree.List(repo)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Branch == fs.Args()[0] {
			if *pathOnly {
				_, err = fmt.Fprintln(a.Out, item.Path)
				return err
			}
			if *editor {
				return runAt(a.Config.Editor, item.Path)
			}
			if *agent {
				return runAt(a.Config.Agent, item.Path)
			}
			return shell(item.Path)
		}
	}
	return fmt.Errorf("worktree for branch %q not found", fs.Args()[0])
}
func (a App) remove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gwt rm <branch>")
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	return worktree.Remove(repo, args[0])
}
func (a App) list(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt list")
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	items, err := worktree.List(repo)
	if err != nil {
		return err
	}
	for _, x := range items {
		fmt.Fprintf(a.Out, "%s\t%s\n", x.Path, x.Branch)
	}
	return nil
}
func (a App) prune() error {
	repos, err := worktree.Repos(a.Dir)
	if err != nil {
		return err
	}
	for _, r := range repos {
		if err := worktree.Prune(r); err != nil {
			return err
		}
	}
	return nil
}
func runAt(command, dir string) error {
	if command == "" {
		return fmt.Errorf("command is not configured")
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
func shell(dir string) error {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	return runAt(sh, dir)
}

func flagsFirst(args []string) []string {
	var flags, values []string
	for _, arg := range args {
		switch arg {
		case "--all", "-e", "-a", "-p":
			flags = append(flags, arg)
		default:
			values = append(values, arg)
		}
	}
	return append(flags, values...)
}
