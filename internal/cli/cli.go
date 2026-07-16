package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/douglasgomes98/gwt/internal/config"
	"github.com/douglasgomes98/gwt/internal/worktree"
	"gopkg.in/yaml.v3"
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
	case "init-config":
		return a.initConfig(args[1:])
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
	tw := tabwriter.NewWriter(a.Out, 0, 0, 2, ' ', 0)
	_, err := fmt.Fprint(tw, `Usage: gwt <command>

Commands:
  add <branch> [base] [-e|-a] [--all]	Create a worktree.
  open <branch|root> [-e|-a]	Open a worktree.
  rm <branch> [--all]	Remove a worktree.
  rm --all	Remove all worktrees in the current root.
  list	List worktrees.
  prune	Prune stale worktrees.
  update [--all]	Update clean roots on the base branch.
  upgrade	Upgrade gwt.
  skill install --agents|--claude	Install the gwt worktree skill for agents.
  init-config	Create a local configuration file.
  checkout-base [--all]	Checkout the base branch in clean roots.
  discard	Discard all local changes in the current root.
  version	Show the version.
  help	Show this help.

Run gwt without a command to open the TUI.
`)
	if err != nil {
		return err
	}
	return tw.Flush()
}

func (a App) initConfig(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gwt init-config")
	}
	data, err := yaml.Marshal(a.Config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	path := filepath.Join(a.Dir, "gwt.yml")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600) // #nosec G304 -- path is fixed to the current working directory.
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("config %s already exists", path)
	}
	if err != nil {
		return fmt.Errorf("create config %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %s: %w", path, err)
	}
	_, err = fmt.Fprintln(a.Out, path)
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
			return runAt(a.Config.Editor, path, path)
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
		return fmt.Errorf("usage: gwt open <branch|root> [-e|-a]")
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
		match := item.Branch == values[0]
		if values[0] == "root" {
			match = item.Primary
		}
		if match {
			if item.Detached {
				return fmt.Errorf("refusing to open detached worktree")
			}
			if flags["-e"] {
				return runAt(a.Config.Editor, item.Path, item.Path)
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
	if len(values) == 0 && flags["--all"] {
		repo, err := worktree.CurrentRepo(a.Dir)
		if err != nil {
			return err
		}
		_, err = worktree.RemoveAll(repo)
		return err
	}
	if len(values) != 1 {
		return fmt.Errorf("usage: gwt rm <branch> [--all] | gwt rm --all")
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
	flags, values, err := parse(args, "--all")
	if err != nil || len(values) != 0 {
		return fmt.Errorf("usage: gwt update [--all]")
	}
	if flags["--all"] {
		repos, err := worktree.Repos(a.Dir)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if err := worktree.ValidateUpdate(repo, a.Config.BaseBranch); err != nil {
				return err
			}
		}
		for _, repo := range repos {
			if err := worktree.Update(repo, a.Config.BaseBranch); err != nil {
				return fmt.Errorf("update --all: result may be partial: %w", err)
			}
		}
		return nil
	}
	repo, err := worktree.CurrentRepo(a.Dir)
	if err != nil {
		return err
	}
	return worktree.Update(repo, a.Config.BaseBranch)
}

func (a App) checkoutBase(args []string) error {
	flags, values, err := parse(args, "--all")
	if err != nil || len(values) != 0 {
		return fmt.Errorf("usage: gwt checkout-base [--all]")
	}
	if flags["--all"] {
		repos, err := worktree.Repos(a.Dir)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if err := worktree.ValidateCheckoutBase(repo); err != nil {
				return err
			}
		}
		for _, repo := range repos {
			if err := worktree.CheckoutBase(repo, a.Config.BaseBranch); err != nil {
				return fmt.Errorf("checkout-base --all: result may be partial: %w", err)
			}
		}
		return nil
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
func runAt(command, dir string, args ...string) error {
	if command == "" {
		return fmt.Errorf("command is not configured")
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], append(parts[1:], args...)...) // #nosec G204,G702 -- command comes from explicit user configuration.
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
