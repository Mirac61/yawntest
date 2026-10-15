# Real-world runs

## Open-source corpus

yawntest runs on 15 well-known Go projects, each pinned to a commit, with nothing configured.
`run --all` generates tests for tested code too, because these projects are well tested and the
invariants must hold either way. So every finding here is either a bug in the project or a
false positive of yawntest, and each one is sorted into one of the two below.

The run is a test with the build tag `realworld`; it writes one summary per project to
[`internal/realworld/testdata`](../internal/realworld/testdata), so the effect of every rule
change shows up as a diff. It runs weekly in CI, and locally with:

```bash
go test -tags realworld ./internal/realworld            # compare
go test -tags realworld ./internal/realworld -update    # rewrite the summaries
```

| Project | Candidates | Hints | Untested error paths | Tests written | Findings | Needs setup |
|---|---:|---:|---:|---:|---:|---:|
| spf13/afero | 6 | 0 | 91 | 12 | 0 | 1 |
| go-chi/chi | 0 | 0 | 4 | 8 | 0 | 2 |
| urfave/cli | 0 | 2 | 0 | 13 | 0 | 1 |
| spf13/cobra | 11 | 2 | 3 | 17 | 5 | 0 |
| shopspring/decimal | 0 | 0 | 7 | 9 | 3 | 0 |
| labstack/echo | 0 | 0 | 18 | 18 | 5 | 2 |
| gin-gonic/gin | 0 | 0 | 3 | 10 | 0 | 1 |
| tidwall/gjson | 0 | 0 | 0 | 12 | 1 | 0 |
| dustin/go-humanize | 0 | 0 | 0 | 25 | 3 | 0 |
| pelletier/go-toml | 0 | 1 | 134 | 6 | 0 | 0 |
| sirupsen/logrus | 0 | 0 | 2 | 7 | 0 | 0 |
| gorilla/mux | 0 | 0 | 28 | 2 | 0 | 0 |
| BurntSushi/toml | 4 | 0 | 8 | 7 | 0 | 0 |
| google/uuid | 0 | 0 | 9 | 6 | 1 | 0 |
| go-playground/validator | 0 | 0 | 3 | 1 | 1 | 0 |

Findings count failed test funcs, not cases. Every run took seconds, except decimal: one
generated test hangs there and only go's 10-minute test timeout ended it.

### Triage

**19 findings: 3 real, 16 false positives.** That's far above the 10 % aimed for, and the
false positives fall into a handful of causes, each with its own fix.

| Finding | Verdict | Why |
|---|---|---|
| go-humanize `FormatFloat("+…", n)`, `FormatInteger` | **real** | A format string with a sign directive yawntest made up panics: "invalid positive sign directive". The doc lists the formats but says nothing about panicking on others, and the format often comes from a template. |
| go-humanize `CommafWithDigits(f, math.MaxInt)` | **real, extreme input** | `strings.Repeat` can't allocate that many zeros. Nobody asks for that many decimals, but the function doesn't guard against it. |
| cobra `MinimumNArgs`, `MaximumNArgs`, `ExactArgs`, `RangeArgs`, `ExactValidArgs` | false positive | They return a func. Two calls give two closures, so "same input, same result" can't hold. |
| echo `TrustLoopback`, `TrustLinkLocal`, `TrustPrivateNet`, `MethodFromQuery` | false positive | Same: option funcs that return a func. |
| decimal `RequireFromString("")` | false positive | Panics by design, its doc says so; `Require` is the `Must` of this package. |
| decimal `NewFromFloat(NaN)`, `NewFromFloat32(NaN)` | false positive | Documented: "NOTE: this will panic on NaN, +/-inf". |
| uuid `MustParse("")` | false positive | A `Must` func panics on bad input, that's its job. |
| validator `asBool("")` | false positive | Its doc says it panics if it can't convert. |
| echo `isValidJSONPCallback("")` | false positive | The empty callback is valid on purpose, the doc says so. The validation rule expected empty input to be rejected. |
| gjson `isDotPiperChar("")` | false positive | An unexported helper whose every caller checks there's a next byte; `""` never reaches it. |
| decimal (hang) | false positive | `NewFromFloatWithExponent` with an exponent of `math.MaxInt32` loops for minutes. Nothing reported it as a finding, the run just took 10 minutes. |

The 4 hints:

| Hint | Verdict | Why |
|---|---|---|
| cli `command_run.go:35`, `:57` auth | false positive | `token` is a token of the command line being split, not a secret. |
| cobra `doc/md_docs.go:113`, `doc/rest_docs.go:126` dates | real | The generated docs stamp `time.Now().Format(…)` in the local time zone. |
| go-toml `cmd/tomltestgen/main.go:105` dates | real | Same, in a code generator. |

Warnings, not findings:

- cobra's own tests fail when run as root (`TestFailGenFishCompletionFile` expects a write to
  fail), so its error paths are skipped. The warning shows the last line of the test output,
  which doesn't say why.
- cli's `docs` is a nested module whose `go.mod` needs updating, so it's checked without type
  information.

Not in the table because they're not wrong: afero, go-toml and mux have many untested error
paths (mostly I/O and parser errors), go-humanize and uuid have no untested candidates at all,
because their maintainers test everything.

## Earlier runs (v0.2)

yawntest was run on seven real Go projects, each a fresh clone of its last commit, nothing
configured. Three are public and shown in full below. The other four are private, so they
appear only as numbers.

### Overview

| Project | Lines of Go | Untested candidates | Hints | Untested error paths | Tests written | Findings | Needs setup |
|---|---:|---:|---:|---:|---:|---:|---:|
| [lifelog](https://github.com/Mirac61/Lifelog) | 2 216 | 13 | 4 | 64 | 13 | 0 | 4 |
| [VentoryGo](https://github.com/Mirac61/VentoryGo) backend | 7 788 | 5 | 0 | 66 | 4 | 0 | 0 |
| [Vlanscape](https://github.com/Mirac61/Vlanscape) backend | 2 180 | 19 | 0 | 34 | 18 | 0 | 2 |
| private backend | 12 034 | 33 | 1 | 193 | 24 | 0 | 15 |
| private service | 3 933 | 14 | 0 | 46 | 14 | 0 | 16 |
| private tool | 1 253 | 5 | 0 | 14 | 5 | 0 | 0 |
| private terminal UI | 6 300 | 17 | 0 | 100 | 17 | **3** | 0 |

Lines exclude generated templ files. A full `yawntest run` took under 2 seconds per project
with a warm build cache. A cold `yawntest check` on lifelog, which runs every test with coverage, took
about 5 seconds.

On every project all generated files compiled, were `gofmt`- and `go vet`-clean, and a second
run produced byte-identical files.

### What the numbers mean

- **Findings: 3, all in one project.** In the terminal UI, three text-padding helpers panic
  when asked to pad to `math.MaxInt` columns: `strings.Repeat` can't allocate that much. The
  widths come from the terminal, so it can't happen in practice. It's still exactly the kind
  of input-range bug yawntest is built to surface.
- **The public projects have no findings.** Their handlers answer bad input with 4xx and their
  pure helpers survive every edge value.
- **Needs setup:** handler methods that use their database. On a zero-valued `Server` they hit
  a nil pointer. yawntest lists them separately and doesn't fail the run. Giving the generated
  test a real receiver turns them into real tests.
- **Untested error paths** are the biggest number everywhere. Nearly all are database and HTTP
  client errors (`return nil, fmt.Errorf("scan …: %w", err)`) that only a failing database or
  network would trigger. That's a coverage gap, not a bug, and a useful list of what a fake
  store or failing client would still test.
- **Skipped:** constructors with params and validators whose input type yawntest can't build.

### lifelog

```text
$ yawntest check
internal/api/day.go
  Server.handleDay  http-handler  line 11  handler signature
internal/api/git.go
  Server.handleGit      http-handler  line 144  handler signature
  Server.handleGitView  http-handler  line 158  handler signature
internal/api/index.go
  Server.handleIndex  http-handler  line 12  handler signature
internal/api/server.go
  Server.handleHealth  http-handler  line 35  handler signature
internal/github/client.go
  Body  json-roundtrip  line 20  has json tags
internal/store/events.go
  PullRequest  json-roundtrip  line 40  has json tags
web/templ/ui/format.go
  DayMonth         pure-func  line 18  basic params, no I/O
  DayMonthYear     pure-func  line 21  basic params, no I/O
  LongDate         pure-func  line 27  basic params, no I/O
  DaysInYear       pure-func  line 32  basic params, no I/O
  BestDaySuffix    pure-func  line 37  basic params, no I/O
  FormatShortDate  pure-func  line 45  basic params, no I/O

13 untested candidates in 7 files.

Hints:
  cmd/lifelog/main.go:23    dates  time.Now().Year() depends on the server's time zone; call .UTC() or .In(loc) first
  cmd/lifelog/main.go:102   dates  time.Now().Year() depends on the server's time zone; call .UTC() or .In(loc) first
  internal/api/git.go:133   dates  time.Now().Year() depends on the server's time zone; call .UTC() or .In(loc) first
  internal/api/index.go:23  dates  time.Now().Year() depends on the server's time zone; call .UTC() or .In(loc) first

Error returns no test reaches:
  cmd/lifelog/sync.go:17            return fmt.Errorf("fetch contributions: %w", err)
  cmd/lifelog/sync.go:20            return fmt.Errorf("insert raw payload: %w", err)
  internal/github/normalize.go:114  return nil, fmt.Errorf("decode contributions: %w", err)
  internal/store/events.go:91       return nil, fmt.Errorf("scan daily total: %w", err)
  …

64 untested error paths.
```

The four date hints all read the "current year" in the server's zone. lifelog's Dockerfile sets
`TZ=Europe/Berlin`, so that's the year its user expects and nothing needs to change. yawntest
can't see deployment config, which is why this is a hint and not a finding: in a container
without `TZ` the year would flip an hour late on January 1st.

```text
$ yawntest run
internal/api/day.go
  ✓ Server.handleDay  http-handler  1 case
internal/api/git.go
  ✓ Server.handleGit      http-handler  1 case
  ✓ Server.handleGitView  http-handler  1 case
internal/api/index.go
  ✓ Server.handleIndex  http-handler  1 case
internal/api/server.go
  ✓ Server.handleHealth  http-handler  1 case
internal/github/client.go
  ✓ Body  json-roundtrip  1 test
internal/store/events.go
  ✓ PullRequest  json-roundtrip  1 test
web/templ/ui/format.go
  ✓ DayMonth         pure-func  fuzz, 8 seeds
  ✓ DayMonthYear     pure-func  fuzz, 8 seeds
  ✓ LongDate         pure-func  fuzz, 8 seeds
  ✓ DaysInYear       pure-func  fuzz, 5 seeds
  ✓ BestDaySuffix    pure-func  fuzz, 8 seeds
  ✓ FormatShortDate  pure-func  fuzz, 8 seeds

Wrote 7 files with 13 tests, skipped 0 candidates.

Needs setup (methods panicked on a zero-valued receiver, give them real dependencies):
  · internal/api  TestYawntest_Server_handleGit_BadInput/empty_request      panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)
  · internal/api  TestYawntest_Server_handleGitView_BadInput/empty_request  panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)
  · internal/api  TestYawntest_Server_handleIndex_BadInput/empty_request    panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)
  · internal/api  TestYawntest_Server_handleHealth_BadInput/empty_request   panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)

0 findings, 4 tests without setup.
```

`handleDay` passes on a zero `Server` because it rejects the empty request before touching
the database. The other four reach `s.db` first.

### VentoryGo backend

The best-tested of the three. Money is integer cents throughout, so no money hints.

```text
$ yawntest run
internal/httperror/error_response.go
  ✓ ErrorResponse  json-roundtrip  1 test
internal/invoice/invoice.go
  ✓ VATBreakdownEntry  json-roundtrip  1 test
internal/invoice/service.go
  ✓ validateVatExempt    validation  1 case (1 expected value TODO)
  ✓ validateForIssue     validation  1 case (1 expected value TODO)
  · validateInvoiceData  validation  skipped, needs inputs yawntest can't build

Wrote 3 files with 4 tests, skipped 1 candidate.

All generated tests pass.
```

### Vlanscape backend

```text
$ yawntest run
internal/domain/types.go
  ✓ IsPassiveType   validation      8 cases (8 expected values TODO)
  ✓ WifiConnection  json-roundtrip  1 test
  ✓ WLAN            json-roundtrip  1 test
  ✓ NetworkModel    json-roundtrip  1 test
internal/domain/validate.go
  ✓ IsIPv4  validation  8 cases (8 expected values TODO)
  ✓ IsCIDR  validation  8 cases (8 expected values TODO)
internal/httpapi/auth.go
  · requireAuth  http-handler  skipped, needs inputs yawntest can't build
internal/httpapi/server.go
  ✓ projectID              http-handler  1 case
  ✓ Server.listProjects    http-handler  1 case
  ✓ Server.createProject   http-handler  1 case
  ✓ Server.getProject      http-handler  1 case
  ✓ Server.updateProject   http-handler  1 case
  ✓ Server.deleteProject   http-handler  1 case
  ✓ Server.shareProject    http-handler  1 case
  ✓ Server.unshareProject  http-handler  1 case
  ✓ Server.getShared       http-handler  1 case
internal/store/store.go
  ✓ ProjectSummary  json-roundtrip  1 test
  ✓ Project         json-roundtrip  1 test
  ✓ SharedProject   json-roundtrip  1 test

Wrote 4 files with 18 tests, skipped 1 candidate.

Needs setup (methods panicked on a zero-valued receiver, give them real dependencies):
  · internal/httpapi  TestYawntest_Server_listProjects_BadInput/empty_request  panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)
  · internal/httpapi  TestYawntest_Server_getShared_BadInput/empty_request     panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)

0 findings, 2 tests without setup.
```

`IsIPv4` and `IsCIDR` get the eight string edge cases as a table, each ending in
`t.Skipf("TODO(yawntest): …")`. Deciding what `""`, `"' OR 1=1 --"` and the 10 000
character string should return turns them into real validator tests: replace the `t.Skipf` line with the
result you expect.

### What the real projects changed in yawntest

Every problem below showed up only on real code, and each fix came with a test that would
have caught it:

| Problem | Seen on | Fix |
|---|---|---|
| Two yawntest files in one package didn't compile, both declared the same helper func | lifelog | Helpers inlined per test; the compile test now puts all fixtures into one package (`62d4e16`) |
| One panicking handler ended the package's test binary, hiding every finding after it | lifelog | Every generated test recovers its own panic (`53f51c4`) |
| Handlers missing their database showed up as findings and failed `run` | lifelog, Vlanscape | Classified as "needs setup", listed apart (`dad5337`, `68c0112`) |
| `darwin.go` and `linux.go` define the same function, so their tests clashed in one build | private terminal UI | Yawntest files copy the source's build constraint (`068f6f7`, `03d2590`) |
| Money hint on `sizeTotal`, `totalOpacity`, `Stats.Total` | VentoryGo, lifelog | `total` no longer counts as money (`d776d3a`) |
| `time.Now()` hint on every deadline, timer and argument: 37 hints, nearly all wrong | all | Only fires on a direct calendar read. 37 hints became 5 (`3a92189`) |

### Reproducing

```bash
go install github.com/Mirac61/yawntest/cmd/yawntest@latest
git clone https://github.com/Mirac61/Lifelog && cd Lifelog
yawntest check
yawntest run
```
