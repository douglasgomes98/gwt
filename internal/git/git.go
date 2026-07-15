package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) // #nosec G204 -- all callers construct Git subcommands from application inputs.
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func IsRepo(dir string) bool {
	_, err := Run(dir, "rev-parse", "--git-dir")
	return err == nil
}
