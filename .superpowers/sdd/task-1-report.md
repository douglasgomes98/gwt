# Task 1 report: strict configuration and bootstrap

## Changed files

- `internal/config/config.go`: made `Load` return `(Config, error)`, added strict YAML decoding, defaults, field validation, and Git branch validation.
- `internal/config/config_test.go`: added default/optional-command and invalid-configuration tests.
- `internal/cli/cli.go`: made `New` receive directory, version, and config; added `version`.
- `internal/cli/cli_test.go`: updated construction and tested `version` with and without extra arguments.
- `cmd/gwt/main.go`: loads configuration before choosing TUI or CLI, then injects it into the selected entrypoint.

## Red/green evidence

Red command:

```sh
go test ./internal/config -run 'TestLoadRejectsInvalidConfig|TestLoadDefaultsAndOptionalCommands' -count=1
go test ./internal/cli -run 'TestVersion|TestHelpListsCommands' -count=1
```

The sandbox could not write Go's shared build cache. Re-running with the required cache permission failed as expected at compile time: the tests required `Load` to return an error and `New` to accept injected arguments.

Green commands:

```sh
go test ./internal/config ./internal/cli -count=1
go test ./... -count=1
git diff --check
```

All commands completed successfully.

## Commit

`fbdba30 feat(config): validate v0 configuration`

## Concerns

None. The configuration validation invokes `git check-ref-format --branch` only for `baseBranch`; editor and agent commands are not checked against `PATH`.
