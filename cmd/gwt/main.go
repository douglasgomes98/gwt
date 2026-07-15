package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes98/gwt/internal/cli"
	"github.com/douglasgomes98/gwt/internal/config"
	"github.com/douglasgomes98/gwt/internal/tui"
)

var version = "dev"

func versionFromBuildInfo(current string, info *debug.BuildInfo) string {
	if current == "dev" && info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return current
}

func main() {
	info, _ := debug.ReadBuildInfo()
	version = versionFromBuildInfo(version, info)
	args := os.Args[1:]
	cwd, _ := os.Getwd()
	cfg, err := config.Load(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gwt:", err)
		os.Exit(1)
	}
	if len(args) == 0 {
		if _, err := tea.NewProgram(tui.New(cwd, cfg)).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := cli.New(os.Stdout, os.Stderr, cwd, version, cfg).Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "gwt:", err)
		os.Exit(1)
	}
}
