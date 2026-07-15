package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes98/gwt/internal/cli"
	"github.com/douglasgomes98/gwt/internal/config"
	"github.com/douglasgomes98/gwt/internal/tui"
)

var version = "dev"

func main() {
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
