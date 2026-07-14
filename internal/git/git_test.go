package git

import (
	"os"
	"os/exec"
	"testing"
)

func TestRunAndIsRepo(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Fatal("temporary directory is not a repository")
	}
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if !IsRepo(dir) {
		t.Fatal("repository not detected")
	}
	if _, err := Run(dir, "not-a-command"); err == nil {
		t.Fatal("expected git error")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}
