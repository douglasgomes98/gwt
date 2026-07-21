---
name: gwt-worktrees
description: Use when an agent needs to organize a task across a Git project group, discover related projects, or create, inspect, enter, or safely remove worktrees with the gwt CLI.
---

# gwt worktrees

Use `gwt` to organize a task as one branch across the projects in its sibling Git group. Use it from a repository root or its sibling-project directory.

## Discover the task and projects

Start by inspecting the current repository's worktrees:

```sh
gwt list
```

To find related worktrees after entering a task checkout, use:

```sh
gwt list --group
```

It lists the current branch (the task) across sibling repositories. Use `gwt list --all`
to discover every existing project and worktree in the group before research or
when the task may affect another app.

After upgrading `gwt`, run `gwt skill update --agents`, `gwt skill update
--claude`, `gwt skill update --codex`, or `gwt skill update --cursor` to
replace the installed copy of this skill with the bundled version.

## Create and enter a task worktree

Research may happen in the current checkout. Before changing files, check whether
the agent is already in a linked worktree:

```sh
git_dir=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
git_common=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
```

If those paths differ, continue in the existing linked worktree. Otherwise, ask
the user for the task branch when it was not provided, then create one worktree
in the target project's primary checkout, capture its printed path, and work
from that path:

```sh
worktree=$(gwt add AG-123)
cd "$worktree"
```

When the task expands from one app to another, use `gwt list --all` to locate
the other app's primary checkout. Run the same `gwt add <branch>` there and
enter its printed path. `gwt list --group` then shows both worktrees for the
task.

For the same branch in every sibling repository, use `--all`; `gwt add` prints one path per repository. Use it only when the task genuinely changes every project in the group.

```sh
gwt add AG-123 --all
```

Do not use `gwt open` to change the current agent's directory: it starts a child shell. Use the path emitted by `gwt add` or the `PATH` column from `gwt list`, then `cd` directly. `gwt open root` opens the primary checkout when a child shell, editor, or agent is needed.

## Safe operations

- Use `gwt list --group` to locate the current task in sibling repositories;
  use `gwt list --all` to inspect the whole group.
- Use `gwt add <branch> [base]`; pass `--all` only when the task genuinely spans sibling repositories.
- Use `gwt prune` only to clean stale Git worktree metadata.
- Run `gwt update` or `gwt checkout-base` only for a clean primary checkout; use `--all` only when every sibling root should receive the operation.
- Never run `gwt rm`, `gwt discard`, `git reset --hard`, or `git clean` without explicit user approval; explain the target paths and data that will be removed first. `gwt discard` also recursively discards changes in initialized submodules.
- Do not remove a primary checkout. `gwt` blocks this, but confirm the branch and path before any deletion.

## Removing worktrees

- `gwt rm <branch>` removes that branch from the current repository.
- `gwt rm <branch> --all` removes that branch from sibling repositories where it exists.
- `gwt rm --all` removes every non-primary worktree from the current repository. Use it only after the user explicitly confirms that root; it rejects the operation if any worktree is detached.
- In the TUI, select a root and choose `rm --all`; confirm the prompt before deletion.
