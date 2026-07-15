# gwt

`gwt` manages Git worktrees in a directory containing multiple repositories.
It is useful when a ticket needs the same branch in more than one project.

Run `gwt` from the directory that groups repositories or from within one of
them. The interface displays worktrees by branch, including dirty and
ahead/behind status.

## Installation

With Go installed:

```sh
go install github.com/douglasgomes98/gwt/cmd/gwt@latest
```

For local development:

```sh
git clone https://github.com/douglasgomes98/gwt.git
cd gwt
make install
```

`go install` places the binary in `GOBIN` or `GOPATH/bin`. Make sure that
directory is in your environment's `PATH`.

## Quick start

Consider this layout:

```text
projects/
  api/
  web/
```

From `projects/`, run `gwt` to open the TUI. From `api/`, CLI commands operate
on that repository; `--all` applies creation to sibling repositories.

```sh
# creates AG-123 in the current repository
gwt add AG-123

# creates AG-123 in api and web
gwt add AG-123 --all

# opens a subshell in the worktree
gwt open AG-123

# removes the current repository's worktree without confirmation
gwt rm AG-123

# removes AG-123 from all sibling repositories with that branch
gwt rm AG-123 --all

# updates the current repository's primary checkout
gwt update

# updates the gwt CLI
gwt upgrade

# checks out the base branch in the primary checkout
gwt checkout-base

# discards all local changes in the primary checkout
gwt discard
```

`gwt open` cannot change its calling shell's directory. It opens a subshell in
the worktree instead; when you exit, you return to the previous directory.

## Commands

| Command | Description |
| --- | --- |
| `gwt` | Opens the TUI. |
| `gwt add <branch> [base] [--all] [-e\|-a]` | Creates a worktree. `--all` creates one in sibling repositories. |
| `gwt open <branch> [-e\|-a]` | Opens a subshell (default), editor, or agent. |
| `gwt rm <branch> [--all]` | Force-removes the current worktree or the same branch from sibling repositories. The primary checkout is never removed. |
| `gwt list` | Lists worktrees in the current repository. |
| `gwt prune` | Runs `git worktree prune` on discovered repositories. |
| `gwt update` | Updates the current repository's clean primary checkout on the base branch. |
| `gwt upgrade` | Updates the installed CLI through Homebrew or Go. |
| `gwt checkout-base` | Checks out the base branch in the current repository's clean primary checkout. |
| `gwt discard` | Discards all local changes in the current repository's primary checkout: tracked, untracked, and ignored. |
| `gwt help` | Shows CLI help. |
| `gwt version` | Shows the binary version. |

The opening flags are mutually exclusive:

- `-e`: uses the configured editor.
- `-a`: uses the configured agent.

## TUI

| Key | Action |
| --- | --- |
| `Space` | Selects a primary checkout or feature. The first feature selection marks all of its worktrees; later presses toggle only the focused row. Detached checkouts cannot be selected. |
| `Enter` | Opens the contextual palette. A single selected root can be opened with the shell, editor, or agent; root maintenance actions are shown only when applicable: `update` requires every selected root to be clean and on the base branch; `checkout-base` requires clean roots; and `discard` appears when a selected root has local changes. Feature actions (`open`, `open -e`, `open -a`, `rm`, `rm --all`, and `prune`) depend on selection and configuration. Choosing `add` opens the branch prompt. `discard` asks for confirmation and removes all local changes from selected roots. |
| `j` / `k` or arrows | Moves focus in the list or palette. |
| `Esc` | Closes the palette without clearing the selection. |
| `q` | Quits. |

Primary checkout names use bold default terminal text; feature worktree names
use cyan. Status colors remain semantic: orange for local changes, green for
ahead, and red for behind.

The TUI preserves the selection and contextual command choice before running
Git operations. During creation, removal, pruning, or updates, it displays a
progress indicator until the operation completes.

## Configuration

Create `gwt.yml` in the directory where you run the command or at
`~/.config/gwt/config.yml`:

```yaml
layout: sibling
baseBranch: main
editor: code
agent: claude
```

All fields are optional. The defaults are `sibling`, `main`, `code`, and
`claude`.

### Layouts

| Layout | Destination for `api` and branch `AG-123` |
| --- | --- |
| `sibling` (default) | `../api.AG-123` |
| `grouped` | `../api.worktrees/api.AG-123` |
| `inside` | `api/.worktrees/AG-123` |

The `inside` layout does not change `.gitignore`; add `.worktrees/` if you do
not want it to appear as untracked content in the primary checkout.

## Development

```sh
make test
make build
make install
make version
```

Tests create temporary Git repositories and exercise creation, removal, and
primary-checkout protection.

## Removal safety

`gwt rm` is deliberately non-interactive and uses forced removal, like the
alias workflow it replaces. Review uncommitted changes before confirming a
removal in the TUI or running `gwt rm`.

## License

[MIT](LICENSE).
