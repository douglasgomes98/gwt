package cli

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed assets/gwt-worktrees/SKILL.md
var gwtWorktreesSkill []byte

var (
	userHomeDir   = os.UserHomeDir
	makeSkillDir  = os.MkdirAll
	openSkillFile = os.OpenFile
)

func (a App) skill(args []string) error {
	if len(args) == 0 || (args[0] != "install" && args[0] != "update") {
		return fmt.Errorf("usage: gwt skill install|update --agents|--claude [--agents|--claude]")
	}
	update := args[0] == "update"
	flags, values, err := parse(args[1:], "--agents", "--claude")
	if err != nil {
		return err
	}
	if len(values) != 0 || (!flags["--agents"] && !flags["--claude"]) {
		return fmt.Errorf("usage: gwt skill install|update --agents|--claude [--agents|--claude]")
	}
	home, err := userHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	paths := skillPaths(home, flags)
	if !update {
		for _, path := range paths {
			if _, err := os.Lstat(path); err == nil {
				return fmt.Errorf("skill already exists at %s", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("inspect skill destination %s: %w", path, err)
			}
		}
	}
	var written []string
	rollback := func() {
		if update {
			return
		}
		for _, path := range written {
			_ = os.Remove(path)
		}
	}
	for _, path := range paths {
		if err := makeSkillDir(filepath.Dir(path), 0750); err != nil {
			rollback()
			return fmt.Errorf("create skill destination: %w", err)
		}
		if err := writeSkill(path, update); err != nil {
			rollback()
			return fmt.Errorf("write skill destination: %w", err)
		}
		written = append(written, path)
	}
	for _, path := range paths {
		if _, err := fmt.Fprintln(a.Out, path); err != nil {
			return err
		}
	}
	return nil
}

func writeSkill(path string, update bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if update {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	file, err := openSkillFile(path, flags, 0600) // #nosec G304 -- path is built from the user home and fixed skill destination.
	if err != nil {
		return err
	}
	if _, err := file.Write(gwtWorktreesSkill); err != nil {
		_ = file.Close()
		if !update {
			_ = os.Remove(path)
		}
		return err
	}
	if err := file.Close(); err != nil {
		if !update {
			_ = os.Remove(path)
		}
		return err
	}
	return nil
}

func skillPaths(home string, flags map[string]bool) []string {
	var paths []string
	if flags["--agents"] {
		paths = append(paths, filepath.Join(home, ".agents", "skills", "gwt-worktrees", "SKILL.md"))
	}
	if flags["--claude"] {
		paths = append(paths, filepath.Join(home, ".claude", "skills", "gwt-worktrees", "SKILL.md"))
	}
	return paths
}
