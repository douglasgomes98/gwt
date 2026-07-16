# Open root alias

## Goal

Allow `gwt open root` to open the current repository's primary checkout,
matching the root-opening behavior available in the TUI.

## Scope

- Treat the exact `root` argument as the primary checkout in `gwt open`.
- Preserve `-e` and `-a`: they open that checkout with the configured editor
  and agent, respectively.
- Keep the existing detached-worktree protection.
- Document the alias in CLI help, the README, and the embedded worktree skill.

## Design

`App.open` will select the primary item when its branch argument is `root`;
otherwise it will retain its existing branch-name lookup. The normal open path
then handles shell, editor, and agent launch without duplication.

## Validation

Add CLI coverage for shell, editor, and agent selection through the alias, run
the focused package tests, then run the repository test and lint targets.
