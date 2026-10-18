# Changelog

yawntest follows [SemVer](https://semver.org) and is pre-1.0, so a minor version can change the
CLI and its output. Each entry says what to adjust.

## Unreleased

### New

- **`install.sh`** installs a release binary, checked against the release's checksums:
  `curl -sSfLO https://raw.githubusercontent.com/Mirac61/yawntest/main/install.sh && sh install.sh`.
- `CONTRIBUTING.md`, `SECURITY.md`, issue templates (bug, false positive, feature) and an
  animated demo in the README: `check`, `run` with findings, `run` after the fixes.
- **`gen --all`, `run --all`** also write tests for code that already has tests. The
  invariants hold either way, and well-tested projects have few untested candidates.
- Findings in `--json` carry the `file`, `line` and `name` of the source the failed test was
  generated for.
- **Measured on 15 open-source projects** (cobra, gin, echo, chi, uuid, …), pinned to commits
  and rerun weekly; each finding is triaged in [docs/real-world.md](docs/real-world.md).
- **Types like `type Slug string`** are fuzzed now, as their basic type converted for the
  call: in pure funcs, validators, roundtrips and JSON fields. Writing the test fixture for
  this, the generated fuzz test found an overflow in it (`Level(l+1)` at 255).
- **`run --fuzztime 10s`** fuzzes every generated fuzz test with Go's fuzzing engine after
  the seeds; failing inputs are saved by go as regression tests.
- **`.yawntest.json`** to exclude paths (`**` globs) and disable patterns, hints or error
  paths for a whole repo. `--config <file>` points at another one.
- **`--format sarif`** and **`--sarif <file>`** for GitHub code scanning and other SARIF
  readers. The action uploads it with `sarif: true`.

### Changed

- **Type information**, with the standard library alone (`go list -export` and `go/types`).
  Purity and error paths now follow calls into other packages of the module, tell methods
  from funcs (`time.Time.After` is not the timer `time.After`), and see what a method on a
  call's result is (`json.NewDecoder(r).Decode` only depends on its input, `exec.Command(…).Run`
  is I/O). `--static` skips it. Packages that don't build fall back to syntax.
- `--format github` keeps to GitHub's limit of 10 annotations per kind and step: the most
  important go first, and a last notice says how many didn't fit. Before, the rest was
  dropped by GitHub without a word.

- **New text output.** Every location is `path:line`, the way `go vet` prints them, so a
  terminal or editor opens it on click. A declaration matching two patterns is one row
  (`pure-func, enum-roundtrip`). Error paths name their func. Every finding comes with the
  `go test` command that reruns just that case, also in JSON as `rerun`. One summary line at
  the end replaces the per-section ones and names the next command. *If you grep the text
  output, switch to `--json`; the text is for people and will keep changing.*
- `check` and `run` show which go test runs are going on (`running tests 3/12…`) on a
  terminal's stderr. In CI logs nothing changes.

### Fixed

- A generated test that hangs no longer holds `run` up for go's 10 minutes: the generated tests
  get 2 minutes, and the one still running is the finding ("still running after 2m0s: this
  input makes it hang"). Before, a timeout or a crash in a goroutine showed up as "build failed".
- When a project's own tests crash, `check` says so and names the test, instead of "tests don't
  build".
- False positives found by running yawntest on 15 real projects (`docs/real-world.md`):
  - Funcs returning a func, a channel or an interface other than `error` (middleware,
    `cobra.MinimumNArgs`) are no pure-func candidates anymore: two closures never compare
    equal, so the determinism check always failed.
  - Funcs that panic on bad input by design, named `Must*`/`Require*` or saying so in their
    doc comment, are no candidates anymore.
  - Validator tables pass `nil`, `""` and empty slices only to exported funcs, and only those get the
    "empty input is rejected" test; an unexported helper's callers have already checked.

- `--format github` gave wrong paths when `GITHUB_WORKSPACE` went through a symlink the
  working directory didn't, like `/var` and `/private/var` on macOS.

## v0.3.0

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
