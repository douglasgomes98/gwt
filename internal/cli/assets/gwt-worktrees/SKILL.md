---
name: gwt-worktrees
description: Use when an agent needs to create, inspect, enter, or safely remove Git worktrees for a task with the gwt CLI.
---

# gwt worktrees

Use `gwt` from a repository root or its sibling-project directory. Start by checking the available worktrees:

```sh
gwt list
```

## Create and enter a task worktree

Create one worktree in the current repository, capture the printed path, then work from that path:

```sh
worktree=$(gwt add AG-123)
cd "$worktree"
```

For the same branch in all sibling repositories, use `--all`; `gwt add` prints one path per repository. Enter only the repository needed for the current task.

```sh
gwt add AG-123 --all
```

Do not use `gwt open` to change the current agent's directory: it starts a child shell. Use the path emitted by `gwt add` or the `PATH` column from `gwt list`, then `cd` directly. `gwt open root` opens the primary checkout when a child shell, editor, or agent is needed.

## Safe operations

- Use `gwt list` before creating a branch or deciding which worktree to enter.
- Use `gwt add <branch> [base]`; pass `--all` only when the task genuinely spans sibling repositories.
- Use `gwt prune` only to clean stale Git worktree metadata.
- Run `gwt update` or `gwt checkout-base` only for a clean primary checkout; use `--all` only when every sibling root should receive the operation.
- Never run `gwt rm`, `gwt discard`, `git reset --hard`, or `git clean` without explicit user approval; explain the target paths and data that will be removed first.
- Do not remove a primary checkout. `gwt` blocks this, but confirm the branch and path before any deletion.

## Removing worktrees

- `gwt rm <branch>` removes that branch from the current repository.
- `gwt rm <branch> --all` removes that branch from sibling repositories where it exists.
- `gwt rm --all` removes every non-primary worktree from the current repository. Use it only after the user explicitly confirms that root; it rejects the operation if any worktree is detached.
- In the TUI, select a root and choose `rm --all`; confirm the prompt before deletion.
