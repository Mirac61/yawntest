# Changelog

yawntest follows [SemVer](https://semver.org) and is pre-1.0, so a minor version can change the
CLI and its output. Each entry says what to adjust.

## v0.3.0 (unreleased)

### New

- **GitHub Action** (`uses: Mirac61/yawntest@v0.3.0`): annotations on the pull request diff and
  the report in the job summary. `only-changed: true` looks only at what the PR changes.
- **`--format github`** prints the result as GitHub Actions annotations. `--format json` is the
  new name for `--json`, which still works.
- **`--since <ref>`** looks only at what changed since the merge base with `ref`, e.g.
  `origin/main`. `--changed` only sees uncommitted work, which is empty in CI.
- **`--static`** for `check` and `gen` runs no tests.
- **roundtrip** pattern: `FormatX`/`ParseX`, `Encode`/`Decode`, `Marshal`/`Unmarshal` and
  similar pairs are fuzzed with "decoding what was encoded gives the input back".
- **enum-roundtrip** pattern: `ParseX(c.String())` must give back `c` for every typed const.
  Catches the enum value someone added and forgot in the parser.
- **idempotent** invariant: `Normalize*`, `Canonical*`, `Clean*` and `Slugify` must not
  change a value they already normalized.
- **`//yawntest:ignore`** in a doc comment leaves a func or type out, anywhere else its line.
- Release binaries for Linux, macOS and Windows, amd64 and arm64.

### Changed

- **Error paths list only returns a unit test can reach**: errors the code makes itself, and
  errors passed on from input-only calls like `strconv.Atoi`. Errors passed on from I/O are left
  out, and packages without any tests are named once instead of listing every return. On
  yawntest's own code this took the list from 39 to 5: three real gaps, now tested, and two
  unreachable returns, now marked with `//yawntest:ignore`. CI runs `yawntest check` on
  yawntest itself. *If you gate CI on `check`, expect fewer error paths, and some new ones for
  `errors.New` and `fmt.Errorf` returns, which weren't listed before.*
- **Purity looks into the package**: a func calling a helper that does I/O is no longer a
  pure-func or validation candidate.
- Flags can now go after the path.
- Go 1.23 is enough, down from 1.27.1.
- `check` and `gen` run each package's tests at most once, several packages at a time.

### Fixed

- A test calling `strings.Split` made the package's own `Split` count as tested.
- A file that doesn't parse aborted the whole run. It's now skipped with a warning.
- Tests that don't build in one package aborted `check`. That package's error paths are now
  skipped with a warning.
- A user test named like `TestYawntestHelper` ran with the generated tests.

## v0.2.0

- `yawntest --version`.
- A versioning note in the README.

## v0.1.0

- First release: `check`, `gen` and `run`; http-handler, json-roundtrip, pure-func and
  validation patterns; money, dates and auth hints; untested error paths; `--changed`, `--json`
  and `--force`.
