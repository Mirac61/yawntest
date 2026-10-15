# Contributing

Thanks for looking into this. Bug reports with a code snippet help most, above all
**false positives**: a candidate, hint, error path or finding that's wrong for your code. Use
the "False positive" issue template; it asks for exactly what's needed to turn it into a test.

## Setup

Go 1.23 or newer and git. No other tools: yawntest uses the standard library only, and that's
a rule, not an accident. Please keep it that way in pull requests.

```bash
go test ./...          # everything, a minute or so
go test -short ./...   # skips the tests that run the go tool, git or the fuzzer
go vet ./...
gofmt -l .             # must print nothing
go run ./cmd/yawntest check ./...   # yawntest checks itself, CI does the same
```

## Golden files

Most tests compare against files in `testdata/`. After a change that's meant to change output,
rewrite them and read the diff before committing:

```bash
go test ./internal/detect -update   # matches, hints, invariants per fixture
go test ./internal/gen -update      # the generated test code
go test ./cmd/yawntest -update      # the CLI end to end on _example
```

`internal/gen` also compiles and runs every fixture with its generated tests, so a template
that produces broken or failing code fails there.

## Adding a rule

1. A fixture in `internal/detect/testdata/` with the code it should match **and** near misses
   it must not match. The golden shows both.
2. The rule in `internal/detect` (one file per pattern or hint). It reads the AST; with type
   information use `Package.objectOf`, and keep the syntax-only path working, since `--static`
   and packages that don't build rely on it.
3. For a pattern, its template in `internal/gen` and a fixture there whose generated tests pass.
4. A line in the README table and in `docs/how-it-works.md`, and an entry in `CHANGELOG.md`.
5. Run the real-world corpus and look at what changed:

   ```bash
   go test -tags realworld ./internal/realworld -update
   git diff internal/realworld/testdata
   ```

   A new finding there is either a bug in that project or a false positive of the rule; say
   which in the pull request (see `docs/real-world.md`).

## The demo

`docs/demo.svg` is rendered from real runs on a copy of `_example`; output changes should
re-render it. `script` keeps the colors, stderr is dropped so the progress line stays out:

```bash
go install ./cmd/yawntest && cp -r _example /tmp/demo && cd /tmp/demo
script -q /dev/null -c 'yawntest check ./... 2>/dev/null' > check.ansi
script -q /dev/null -c 'yawntest run ./... 2>/dev/null' > run.ansi
# fix the four findings in shop/order.go and shop/status.go, then
script -q /dev/null -c 'yawntest run ./... 2>/dev/null' > green.ansi
cd - && python3 docs/render-demo.py docs/demo.svg 'yawntest check' /tmp/demo/check.ansi \
  'yawntest run' /tmp/demo/run.ansi 'yawntest run      # after fixing the 4 bugs' /tmp/demo/green.ansi
```

The renderer is Python's standard library only. Look at the result in a browser before
committing.

## Commits and pull requests

Small pull requests, one topic each. Commit messages say what changed and why, in plain
sentences. CI runs the tests on Linux, macOS and the oldest supported Go.
