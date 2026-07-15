# Semantic release design

## Goal

Publish `gwt` automatically from `main` with Semantic Versioning tags, GitHub
release binaries, a personal Homebrew tap, and a Go-installable module.

## Scope

- Start at `v0.1.0`.
- Derive versions from Conventional Commits: `fix:` is patch, `feat:` is minor,
  and `!` or `BREAKING CHANGE:` is major. Other types do not release.
- Correct the Go module path and internal imports to
  `github.com/douglasgomes98/gwt`, matching the public repository and the
  documented `go install` command.
- Run release automation only after the existing Go checks pass on `main`.
- Produce GitHub release archives for supported macOS, Linux, and Windows
  targets, plus checksums.
- Update the `douglasgomes98/homebrew-tap` Cask from GoReleaser.
- Add a local pull-request rule: titles must be Conventional Commits; with
  squash merge, the title is the release-relevant commit message.
- Document Go and Homebrew installation and the release convention.

## Architecture

One GitHub Actions release workflow runs on pushes to `main`.

1. Check out complete history and run the existing Go checks.
2. Run Semantic Release with an exact, lockfile-managed Node dependency set.
   It analyzes commits and creates the next `vX.Y.Z` tag only when a release is
   warranted.
3. Run GoReleaser in the same job against that tag. It builds archives,
   publishes the GitHub release, and commits the generated Cask to the tap.

The release workflow has `contents: write` for tags and GitHub releases.
`TAP_GITHUB_TOKEN` is a repository secret used only by GoReleaser to update
the separate tap repository. The standard `GITHUB_TOKEN` is not used to write
to the tap.

## Installation contract

The tag is the single source of truth:

```sh
go install github.com/douglasgomes98/gwt/cmd/gwt@vX.Y.Z
brew tap douglasgomes98/tap
brew install --cask gwt
```

`gwt upgrade` continues to select Homebrew when the running binary belongs to
the `gwt` Cask; otherwise it runs Go installation.

## Failure handling

- CI failure prevents version calculation and publication.
- No qualifying commit exits successfully without a release.
- A missing or invalid `TAP_GITHUB_TOKEN` fails publication instead of silently
  publishing an incomplete release.
- A failed release remains diagnosable from its GitHub Actions run; operators
  fix configuration and rerun only after checking the tag/release state to
  avoid duplicate assets.

## Verification

- Add focused tests for the corrected Go module/import path where compilation
  covers it.
- Validate Semantic Release in dry-run mode using fixture commit history or
  configuration checks.
- Run GoReleaser in snapshot mode to validate archives and Cask rendering
  without publishing.
- Run `make lint`, `make test`, `make coverage`, and `make build`.

## Non-goals

- Homebrew core submission.
- Pre-release channels, signing, notarization, Docker images, or automatic
  changelog commits.
