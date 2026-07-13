package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/douglasgomes/gwt/internal/config"
	"github.com/douglasgomes/gwt/internal/git"
)

type Item struct {
	Repo, Branch, Path string
	Dirty              bool
	Changes            int
	Ahead, Behind      int
}

func Root(cwd string) string {
	if !git.IsRepo(cwd) {
		return cwd
	}
	items, err := ListFast(cwd)
	if err != nil || len(items) == 0 {
		return cwd
	}
	return filepath.Dir(items[0].Path)
}

func Repos(cwd string) ([]string, error) {
	root := Root(cwd)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var repos []string
	if primary(root) {
		repos = append(repos, root)
	}
	for _, e := range entries {
		if e.IsDir() && primary(filepath.Join(root, e.Name())) {
			repos = append(repos, filepath.Join(root, e.Name()))
		}
	}
	return repos, nil
}

func CurrentRepo(cwd string) (string, error) {
	if !git.IsRepo(cwd) {
		return "", fmt.Errorf("%s is not a Git repository", cwd)
	}
	items, err := ListFast(cwd)
	if err != nil || len(items) == 0 {
		return "", fmt.Errorf("cannot resolve primary checkout for %s", cwd)
	}
	return items[0].Path, nil
}

func List(repo string) ([]Item, error) {
	items, err := ListFast(repo)
	if err != nil {
		return nil, err
	}
	for i := range items {
		status, _ := git.Run(items[i].Path, "status", "--porcelain")
		items[i].Dirty = strings.TrimSpace(status) != ""
		if trimmed := strings.TrimSpace(status); trimmed != "" {
			items[i].Changes = strings.Count(trimmed, "\n") + 1
		}
		counts, err := git.Run(items[i].Path, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
		if err == nil {
			fmt.Sscanf(counts, "%d %d", &items[i].Behind, &items[i].Ahead)
		}
	}
	return items, nil
}

func ListFast(repo string) ([]Item, error) {
	out, err := git.Run(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var items []Item
	var current *Item
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			items = append(items, Item{Repo: filepath.Base(repo), Path: strings.TrimPrefix(line, "worktree ")})
			current = &items[len(items)-1]
		case strings.HasPrefix(line, "branch ") && current != nil:
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	return items, nil
}

func primary(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

func Path(repo, branch string, c config.Config) string {
	parent, name := filepath.Dir(repo), filepath.Base(repo)
	switch c.Layout {
	case "inside":
		return filepath.Join(repo, ".worktrees", branch)
	case "grouped":
		return filepath.Join(parent, name+".worktrees", name+"."+branch)
	default:
		return filepath.Join(parent, name+"."+branch)
	}
}

func Add(repo, branch, base string, c config.Config) (string, error) {
	if strings.TrimSpace(branch) == "" {
		return "", fmt.Errorf("branch is required")
	}
	path := Path(repo, branch, c)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if _, err := git.Run(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err := git.Run(repo, "worktree", "add", path, branch)
		return path, err
	}
	_, err := git.Run(repo, "worktree", "add", "-b", branch, path, base)
	return path, err
}

func Remove(repo, branch string) error {
	items, err := List(repo)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Branch == branch {
			same, _ := filepath.EvalSymlinks(item.Path)
			if same == "" {
				same = item.Path
			}
			main, _ := CurrentRepo(repo)
			if same == main {
				return fmt.Errorf("refusing to remove primary checkout")
			}
			_, err := git.Run(repo, "worktree", "remove", "--force", "--force", item.Path)
			return err
		}
	}
	return fmt.Errorf("worktree for branch %q not found", branch)
}

func Fetch(repo, base string) error { _, err := git.Run(repo, "fetch", "origin", base); return err }
func Prune(repo string) error       { _, err := git.Run(repo, "worktree", "prune"); return err }
