# Semantic Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Publish tagged Go releases, GitHub archives, and a source-building Homebrew formula.

**Architecture:** Semantic Release analyzes Conventional Commits on main and invokes a release script. The script runs GoReleaser and updates the checked-out tap formula from the release tag.

**Tech Stack:** Go 1.26, GitHub Actions, Semantic Release 25.0.6, GoReleaser v2, POSIX shell.

## Constraints

- First release: v0.1.0.
- fix: patch; feat: minor; ! or BREAKING CHANGE: major.
- Module path: github.com/douglasgomes98/gwt.
- Tap: douglasgomes98/homebrew-tap; TAP_GITHUB_TOKEN has write access only there.
- Formula builds source; no Cask, Apple signing, notarization, Docker, prereleases, or changelog commits.

### Task 1: Correct the module path

**Files:** go.mod; all first-party Go imports in cmd/ and internal/.

- [ ] Run `test "$(sed -n '1p' go.mod)" = 'module github.com/douglasgomes98/gwt'`; it must fail.
- [ ] Replace only `github.com/douglasgomes/gwt` with `github.com/douglasgomes98/gwt`.
- [ ] Run `go mod tidy && go test ./... && ! rg -n 'github\.com/douglasgomes/gwt' --glob '*.go' --glob go.mod`; it must pass.
- [ ] Commit `fix: align Go module path`.

### Task 2: Add the PR title rule

**Files:** .agents/rules/pull-requests.md.

- [ ] Confirm `rg -n 'squash|Conventional Commit|title' .agents/rules/pull-requests.md` prints nothing.
- [ ] Add: “Use a Conventional Commit as every PR title. With squash merge, the PR title becomes the commit on main that Semantic Release analyzes. Use docs:, chore:, ci:, refactor:, or test: for changes that must not release.”
- [ ] Verify the three phrases with rg and commit `docs: define release-safe PR titles`.

### Task 3: Configure semantic release and archives

**Files:** package.json, package-lock.json, release.config.mjs, .goreleaser.yaml.

- [ ] Confirm each file is absent with `test -f`.
- [ ] Create package.json with private devDependencies `semantic-release: 25.0.6` and `@semantic-release/exec: 7.1.0`; run `npm install --package-lock-only`.
- [ ] Configure Semantic Release for branch main, commit analyzer, release notes generator, and exec publisher calling `./scripts/release.sh` with the next git tag.
- [ ] Configure GoReleaser v2 with Darwin, Linux, and Windows builds from ./cmd/gwt; inject `main.version={{ .Version }}`; create matching archives, checksums.txt, and exclude docs/test/ci/chore from generated changelog. Do not configure Homebrew.
- [ ] Run `npm ci && npx semantic-release --dry-run && goreleaser check && goreleaser release --snapshot --clean`.
- [ ] Commit `build: add semantic release tooling`.

### Task 4: Implement and test formula publication

**Files:** scripts/release.sh, scripts/release_test.sh.

- [ ] Write release_test.sh first. It creates a temporary real Git tap, a fake goreleaser, and a fake curl yielding fixture bytes; it invokes release.sh v0.1.0 with TAP_DIR set. It asserts Formula/gwt.rb contains the tag source URL, fixture SHA, `depends_on "go" => :build`, and `std_go_args`; it also asserts a `Brew formula update for gwt v0.1.0` commit.
- [ ] Run `sh scripts/release_test.sh`; it must fail because release.sh does not exist.
- [ ] Implement release.sh: require TAP_DIR and tag; calculate the SHA-256 of https://github.com/douglasgomes98/gwt/archive/refs/tags/<tag>.tar.gz; run `goreleaser release --clean`; write Formula/gwt.rb using that URL/SHA, a Go build dependency, `std_go_args(ldflags: "-s -w -X main.version=#{version}")`, and a version command test; add, commit, and push the formula. Use the checked-out tap credential; never put tokens in shell text or command arguments.
- [ ] Make both scripts executable; run the test again and commit `build: publish Homebrew source formula`.

### Task 5: Automate and document

**Files:** .github/workflows/release.yml, README.md.

- [ ] Confirm release.yml is absent.
- [ ] Create a main-push workflow with contents: write. Check out complete source history; check out douglasgomes98/homebrew-tap into .homebrew-tap with TAP_GITHUB_TOKEN; set up Go, Node LTS, and GoReleaser v2; run make test, npm ci, and npx semantic-release with GITHUB_TOKEN and TAP_DIR pointing at the tap checkout.
- [ ] Document `go install github.com/douglasgomes98/gwt/cmd/gwt@vX.Y.Z`, `brew tap douglasgomes98/tap`, and `brew install gwt`; document Conventional Commit squash-merge titles and that the public tap plus fine-grained TAP_GITHUB_TOKEN must exist before the first release.
- [ ] Run `git diff --check; sh -n scripts/release.sh scripts/release_test.sh; sh scripts/release_test.sh; goreleaser check; make lint; make test; make coverage; make build`.
- [ ] Commit `ci: automate semantic releases`.

## Final verification

- [ ] Worktree is clean.
- [ ] README and local PR rule agree.
- [ ] Do not push the branch or create the tap automatically.
