# Repository Guidelines

## Project Structure & Module Organization

`gwt` is a Go CLI and terminal UI for managing Git worktrees across sibling
repositories. The executable entry point is `cmd/gwt/main.go`. Keep reusable
application code under `internal/`:

- `internal/cli/` parses and runs command-line subcommands.
- `internal/tui/` contains the Bubble Tea model and terminal rendering.
- `internal/worktree/` discovers, creates, and removes worktrees.
- `internal/git/` wraps Git invocations; `internal/config/` loads `gwt.yml`.

Put tests beside the code they cover, using `*_test.go`. Do not add external
assets or a public package unless the command genuinely needs one.

## Build, Test, and Development Commands

Run these from the repository root:

```sh
make deps       # download Go module dependencies
make lint       # run the GolangCI-Lint quality gate
make test       # go vet ./... followed by go test ./...
make build      # build bin/gwt with version metadata
make coverage   # write coverage.out and print function coverage
make install    # install gwt into GOBIN or GOPATH/bin
make version    # print the version derived from Git
```

Use `go test ./internal/tui` (or another package path) for a focused test run.
`coverage.out` and `bin/gwt` are generated files; do not commit them.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs for indentation and `gofmt` on every changed
Go file. Keep package names lowercase and short (`worktree`, `config`), export
only APIs used across packages, and name tests `TestBehavior`. Prefer small,
direct functions and standard-library facilities. Wrap command failures with
context, as `internal/git.Run` does.

## Testing Guidelines

Follow TDD: write or update unit tests before implementing behavior changes.
Tests use Go's built-in `testing` package. Cover all observable behavior and
error paths, especially Git operations and destructive-worktree safeguards.
Use `t.TempDir()` and temporary Git repositories rather than local checkout
paths; use `t.Setenv()` for environment-dependent behavior. Keep project
coverage above 90%; run `make lint`, `make test`, and `make coverage` before
opening a pull request.

## Documentation

For every added or modified feature, review the README and update it when
needed so new users can discover and use the behavior.

## Commit & Pull Request Guidelines

Follow the existing Conventional Commit style: `fix(tui): open shell in selected
worktree`, `feat(cli): add prune command`, or `refactor(worktree): simplify scan`.
Keep commits focused. Pull requests should state the user-visible change, link
the relevant issue when available, include test results, and attach a terminal
capture for TUI-facing changes. Call out any change that can remove worktrees
or alter Git commands. Create every change in an isolated worktree on a
dedicated branch; never push directly to `main`, and merge through a pull
request.

## Configuration & Safety

Configuration is loaded from `gwt.yml` in the working directory or
`~/.config/gwt/config.yml`. Never put personal paths or credentials in examples.
Treat `gwt rm` as destructive: preserve the protection against removing the
primary checkout and test any change to removal behavior carefully.
