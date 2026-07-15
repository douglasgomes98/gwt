# Pull requests

Create every change in an isolated Git worktree on a dedicated branch. Do not
push directly to `main`; push the branch and merge through a pull request.

Use a Conventional Commit as every PR title. With squash merge, the PR title
becomes the commit on `main` that Semantic Release analyzes. Use `docs:`,
`chore:`, `ci:`, `refactor:`, or `test:` when the change must not release.
