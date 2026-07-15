package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/worktree"
)

var executable = os.Executable
var command = exec.Command

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
		return a.prune(args[1:])
	case "update":
		return a.update(args[1:])
	case "upgrade":
		return a.upgrade(args[1:])
	case "skill":
		return a.skill(args[1:])
	case "checkout-base":
		return a.checkoutBase(args[1:])
	case "discard":
		return a.discard(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a App) upgrade(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt upgrade")
	}
	path, err := executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	prefix, err := command("brew", "--prefix", "gwt").Output()
	name, values := "go", []string{"install", "github.com/douglasgomes98/gwt/cmd/gwt@latest"}
	if err == nil && strings.HasPrefix(path, strings.TrimSpace(string(prefix))+string(filepath.Separator)) {
		name, values = "brew", []string{"upgrade", "gwt"}
	}
	cmd := command(name, values...)
	cmd.Stdout, cmd.Stderr = a.Out, a.Err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	return nil
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
  open <branch> [-e|-a]                Open a worktree.
  rm <branch> [--all]                  Remove a worktree.
  list                                 List worktrees.
  prune                                Prune stale worktrees.
  update                               Update the current root.
  upgrade                              Upgrade gwt.
  skill install --agents|--claude       Install the gwt worktree skill for agents.
  checkout-base                        Checkout the base branch in the current root.
  discard                              Discard all local changes in the current root.
  version                              Show the version.
  help                                 Show this help.

Run gwt without a command to open the TUI.
`)
	return err
}

func (a App) add(args []string) error {
	flags, values, err := parse(args, "--all", "-e", "-a")
	if err != nil {
		return err
	}
	if exclusive(flags, "--all", "-e", "-a") {
		return fmt.Errorf("usage: gwt add <branch> [base] [-e|-a] [--all]")
	}
	if len(values) < 1 || len(values) > 2 {
		return fmt.Errorf("usage: gwt add <branch> [base] [-e|-a] [--all]")
	}
	base := a.Config.BaseBranch
	if len(values) == 2 {
		base = values[1]
	}
	var repos []string
	if flags["--all"] {
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
		path, err := worktree.Add(repo, values[0], base, a.Config)
		if err != nil {
			if flags["--all"] {
				return fmt.Errorf("add --all: result may be partial: %w", err)
			}
			return err
		}
		if _, err := fmt.Fprintln(a.Out, path); err != nil {
			return err
		}
		if flags["-e"] {
			return runAt(a.Config.Editor, path)
		}
		if flags["-a"] {
			return runAt(a.Config.Agent, path)
		}
	}
	return nil
}
func (a App) open(args []string) error {
	flags, values, err := parse(args, "-e", "-a")
	if err != nil {
		return err
	}
	if exclusive(flags, "-e", "-a") || len(values) != 1 {
		return fmt.Errorf("usage: gwt open <branch> [-e|-a]")
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
		if item.Branch == values[0] {
			if item.Detached {
				return fmt.Errorf("refusing to open detached worktree")
			}
			if flags["-e"] {
				return runAt(a.Config.Editor, item.Path)
			}
			if flags["-a"] {
				return runAt(a.Config.Agent, item.Path)
			}
			return shell(item.Path)
		}
	}
	return fmt.Errorf("worktree for branch %q not found", values[0])
}
func (a App) remove(args []string) error {
	flags, values, err := parse(args, "--all")
	if err != nil {
		return err
	}
	if len(values) != 1 {
		return fmt.Errorf("usage: gwt rm <branch> [--all]")
	}
	if !flags["--all"] {
		repo, err := worktree.CurrentRepo(a.Dir)
		if err != nil {
			return err
		}
		return worktree.Remove(repo, values[0])
	}
	repos, err := worktree.Repos(a.Dir)
	if err != nil {
		return err
	}
	type target struct {
		repo string
		item worktree.Item
	}
	var targets []target
	for _, repo := range repos {
		item, err := worktree.Find(repo, values[0])
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				continue
			}
			return err
		}
		targets = append(targets, target{repo, item})
	}
	if len(targets) == 0 {
		return fmt.Errorf("worktree for branch %q not found", values[0])
	}
	for _, target := range targets {
		if target.item.Primary {
			return fmt.Errorf("refusing to remove primary checkout")
		}
		if target.item.Detached {
			return fmt.Errorf("refusing to remove detached worktree")
		}
	}
	for _, target := range targets {
		if err := worktree.Remove(target.repo, values[0]); err != nil {
			return fmt.Errorf("rm --all: result may be partial: %w", err)
		}
	}
	return nil
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
	tw := tabwriter.NewWriter(a.Out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PATH\tBRANCH\tSTATUS"); err != nil {
		return err
	}
	for _, x := range items {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", x.Path, displayBranch(x), worktree.Status(x)); err != nil {
			return err
		}
	}
	return tw.Flush()
}
func (a App) prune(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt prune")
	}
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
func (a App) update(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt update")
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	return worktree.Update(repo, a.Config.BaseBranch)
}

func (a App) checkoutBase(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt checkout-base")
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	return worktree.CheckoutBase(repo, a.Config.BaseBranch)
}

func (a App) discard(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt discard")
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	return worktree.Discard(repo)
}

func parse(args []string, allowed ...string) (map[string]bool, []string, error) {
	known, flags := map[string]bool{}, map[string]bool{}
	for _, name := range allowed {
		known[name] = true
	}
	var values []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if !known[arg] {
				return nil, nil, fmt.Errorf("unknown flag %q", arg)
			}
			flags[arg] = true
		} else {
			values = append(values, arg)
		}
	}
	return flags, values, nil
}

func exclusive(flags map[string]bool, names ...string) bool {
	count := 0
	for _, name := range names {
		if flags[name] {
			count++
		}
	}
	return count > 1
}

func displayBranch(item worktree.Item) string {
	if item.Detached {
		return "(detached)"
	}
	return item.Branch
}
func runAt(command, dir string) error {
	if command == "" {
		return fmt.Errorf("command is not configured")
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...) // #nosec G204,G702 -- command comes from explicit user configuration.
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
