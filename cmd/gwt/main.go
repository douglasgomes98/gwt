package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/douglasgomes/gwt/internal/cli"
	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/tui"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		fmt.Println(version)
		return
	}
	if len(args) == 0 {
		cwd, _ := os.Getwd()
		if _, err := tea.NewProgram(tui.New(cwd, config.Load(cwd))).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := cli.New(os.Stdout, os.Stderr).Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "gwt:", err)
		os.Exit(1)
	}
}
