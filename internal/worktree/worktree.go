package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/douglasgomes98/gwt/internal/config"
	"github.com/douglasgomes98/gwt/internal/git"
)

type Item struct {
	Repo, Branch, Path string
	Primary, Detached  bool
	Dirty              bool
	Changes            int
	Ahead, Behind      int
}

func Status(item Item) string {
	var parts []string
	if item.Changes > 0 {
		parts = append(parts, fmt.Sprintf("%d files changed", item.Changes))
	}
	if item.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("ahead %d", item.Ahead))
	}
	if item.Behind > 0 {
		parts = append(parts, fmt.Sprintf("behind %d", item.Behind))
	}
	if len(parts) == 0 {
		return "(clean)"
	}
	return "(" + strings.Join(parts, " · ") + ")"
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
			if _, err := fmt.Sscanf(counts, "%d %d", &items[i].Behind, &items[i].Ahead); err != nil {
				return nil, fmt.Errorf("parse branch counts: %w", err)
			}
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
			items = append(items, Item{Repo: filepath.Base(repo), Path: strings.TrimPrefix(line, "worktree "), Primary: len(items) == 0})
			current = &items[len(items)-1]
		case strings.HasPrefix(line, "branch ") && current != nil:
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "detached" && current != nil:
			current.Detached = true
		}
	}
	return items, nil
}

func Find(repo, branch string) (Item, error) {
	items, err := List(repo)
	if err != nil {
		return Item{}, err
	}
	for _, item := range items {
		if item.Branch == branch {
			return item, nil
		}
	}
	return Item{}, fmt.Errorf("worktree for branch %q not found", branch)
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
	items, err := ListFast(repo)
	if err != nil {
		return "", err
	}
	if len(items) > 0 && items[0].Detached {
		return "", fmt.Errorf("refusing to add from detached primary checkout")
	}
	path := Path(repo, branch, c)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { // #nosec G301 -- Git requires a traversable worktree parent derived from a trusted repository path.
		return "", err
	}
	if _, err := git.Run(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err := git.Run(repo, "worktree", "add", path, branch)
		return path, err
	}
	_, err = git.Run(repo, "worktree", "add", "-b", branch, path, base)
	return path, err
}

func Remove(repo, branch string) error {
	item, err := Find(repo, branch)
	if err != nil {
		return err
	}
	if item.Primary {
		return fmt.Errorf("refusing to remove primary checkout")
	}
	if item.Detached {
		return fmt.Errorf("refusing to remove detached worktree")
	}
	_, err = git.Run(repo, "worktree", "remove", "--force", "--force", item.Path)
	return err
}

func Fetch(repo, base string) error { _, err := git.Run(repo, "fetch", "origin", base); return err }
func Prune(repo string) error       { _, err := git.Run(repo, "worktree", "prune"); return err }

func Update(path, base string) error {
	status, err := git.Run(path, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("root has uncommitted changes")
	}
	branch, err := git.Run(path, "branch", "--show-current")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) != base {
		return fmt.Errorf("root must be on %s", base)
	}
	if err := Fetch(path, base); err != nil {
		return err
	}
	_, err = git.Run(path, "merge", "--ff-only", "origin/"+base)
	return err
}

func CheckoutBase(path, base string) error {
	status, err := git.Run(path, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("root has uncommitted changes")
	}
	branch, err := git.Run(path, "branch", "--show-current")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("refusing to checkout detached root")
	}
	_, err = git.Run(path, "checkout", base)
	return err
}

func Discard(path string) error {
	branch, err := git.Run(path, "branch", "--show-current")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("refusing to discard detached root")
	}
	if _, err := git.Run(path, "reset", "--hard"); err != nil {
		return err
	}
	_, err = git.Run(path, "clean", "-fdx")
	return err
}
