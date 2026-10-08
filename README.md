# lazytest

[![CI](https://github.com/Mirac61/lazytest/actions/workflows/ci.yml/badge.svg)](https://github.com/Mirac61/lazytest/actions/workflows/ci.yml)

**Writes the boring tests for your Go code.** lazytest reads your source, recognizes common
shapes (HTTP handlers, JSON structs, pure functions, validators) and generates tests for them.
Offline, deterministic, no AI, standard library only.

It never guesses what your code *should* return. It only checks things that hold no matter
what your business logic is: no panic, bad input never gives a 5xx, JSON survives a round
trip, the same input gives the same output, split amounts add up to the total. When a
generated test fails on your current code, that's a **finding**, a likely bug.

```text
$ lazytest --run ./...
shop/order.go
  ✓ Order        json-roundtrip  1 test
  ✓ CreateOrder  http-handler    3 cases
  ✓ Split        pure-func       fuzz, 9 seeds, parts sum to total
  ✓ Initial      pure-func       fuzz, 8 seeds
shop/user.go
  ✓ ValidateEmail  validation  9 cases (8 expected values TODO)

Wrote 2 files with 5 tests, skipped 0 candidates.

Findings (generated tests that fail on the current code):
  ⚠ shop  TestLazytest_CreateOrder_BadInput/empty_request    status = 500, want < 500 for bad input
  ⚠ shop  TestLazytest_CreateOrder_BadInput/invalid_json     status = 500, want < 500 for bad input
  ⚠ shop  TestLazytest_CreateOrder_BadInput/empty_json_body  status = 500, want < 500 for bad input
  ⚠ shop  FuzzLazytest_Split/seed#5                          Split(100, 3) parts add up to 99, want the total 100
  ⚠ shop  FuzzLazytest_Split/seed#6                          Split(101, 2) parts add up to 100, want the total 101
  ⚠ shop  FuzzLazytest_Split/seed#7                          Split(1, 3) parts add up to 0, want the total 1
  ⚠ shop  FuzzLazytest_Split/seed#8                          Split(999, 7) parts add up to 994, want the total 999
  ⚠ shop  FuzzLazytest_Initial/seed#0                        panic: runtime error: slice bounds out of range [:1] with length 0

8 findings.
```

That output is real: it's lazytest run on [`_example/`](_example/), a small shop package with
three planted bugs. All three are found.

## Install

```bash
go install github.com/Mirac61/lazytest/cmd/lazytest@latest
```

Needs the Go version from [`go.mod`](go.mod) and `git` for `--changed`.

## Usage

```bash
lazytest [flags] [path]     # path defaults to ".", "./..." works too
```

| Flag | What it does |
|---|---|
| *(none)* | Generate tests for every untested candidate below `path` |
| `--check` | Write nothing. List untested candidates, domain hints and error paths no test reaches |
| `--run` | Generate, then run the generated tests and report failures as findings |
| `--changed` | Only look at code changed since the last commit (`git diff HEAD` plus untracked files) |
| `--force` | Overwrite existing lazytest files |
| `--json` | Print the result as JSON instead of text |

Flags go before the path (`lazytest --check .`, not `lazytest . --check`).

**Exit codes:** `0` all good, `1` something to look at (untested candidates or error paths
with `--check`, findings with `--run`), `2` lazytest itself failed. Hints and tests that only
need setup never cause a `1`, so `lazytest --check` and `lazytest --run` can gate CI.

## What it generates

| Pattern | Detected when | Generated test |
|---|---|---|
| `http-handler` | `func(http.ResponseWriter, *http.Request)`, function or method, or a constructor returning `http.HandlerFunc` | Empty request must not give a 5xx. If the body is decoded as JSON: invalid and empty bodies must give a 4xx |
| `json-roundtrip` | Exported struct with `json` tags | Fills every field with a non-zero value, marshals, unmarshals, compares with `reflect.DeepEqual` |
| `pure-func` | Exported function, only basic-typed params (numbers, strings, bools, `time.Time`, slices of them), has a result, no I/O in its body | Native Go fuzz test seeded with edge values. Checks no panic and that two calls return the same result |
| `validation` | Returns only `error` or `bool` and is named `Validate*`, `IsValid*`, `Check*`, or takes a single string | Table of edge inputs that must not panic. The expected result is a `TODO(lazytest)` with `t.Skip`, since that's your decision. Named validators also get "empty input is rejected" |

Each source file gets one test file next to it, `order.go` → `order_lazytest_test.go`, in the
same package. User-written test files are never touched. A second run without `--force`
changes nothing.

### A generated test

From the example, the handler test that found the 500:

```go
// lazytest: http-handler / bad input never gives 5xx, invalid JSON gives 4xx
func TestLazytest_CreateOrder_BadInput(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		body    string
		want4xx bool
	}{
		{name: "empty request", method: http.MethodGet},
		{name: "invalid json", method: http.MethodPost, body: "{", want4xx: true},
		{name: "empty json body", method: http.MethodPost, want4xx: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic: %v", r)
				}
			}()
			handler := CreateOrder
			request := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()

			handler(recorder, request)

			status := recorder.Code
			if status >= 500 {
				t.Errorf("status = %d, want < 500 for bad input", status)
			}
			if test.want4xx && status < 400 {
				t.Errorf("status = %d, want 4xx", status)
			}
		})
	}
}
```

And the validator test, where lazytest leaves the business decision to you:

```go
// lazytest: validation / no panic; expected results are up to you
func TestLazytest_ValidateEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "space", input: " "},
		{name: "single char", input: "a"},
		{name: "very long", input: strings.Repeat("x", 10000)},
		{name: "unicode", input: "ünïcødé"},
		{name: "emoji", input: "😀"},
		{name: "null byte", input: "\x00"},
		{name: "sql injection", input: "' OR 1=1 --"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// (panic recovery omitted here)
			err := ValidateEmail(test.input)
			t.Skipf("TODO(lazytest): expected result for %s; got %v", test.name, err)
		})
	}
}
```

Replace the `t.Skipf` line with the result you expect and the case becomes a real test.

### Invariants

For a few shapes lazytest checks more than "no panic, deterministic":

- **Parts sum to total:** a pure func taking an integer amount first (`total`, `amount`,
  `price`, …) and returning a slice of the same type, like `Split(total int64, parts int) []int64`.
  Gets extra seeds that don't divide evenly (100/3, 999/7).
- **End not before start:** a pure func returning `(start, end time.Time)`, or two times from
  a func named like `WeekRange`.

## Checking without writing

```text
$ lazytest --check ./...
shop/order.go
  Order        json-roundtrip  line 11  has json tags
  CreateOrder  http-handler    line 20  handler signature, decodes JSON body
  Split        pure-func       line 30  basic params, no I/O
  Initial      pure-func       line 42  basic params, no I/O
shop/user.go
  ValidateEmail  validation  line 13  name Validate*, returns error

5 untested candidates in 2 files.

Hints:
  shop/order.go:15  money  Price is a float64; floats can't hold cents exactly, use integer cents or a decimal type
  shop/order.go:52  dates  time.Now().Year() depends on the server's time zone; call .UTC() or .In(loc) first
  shop/user.go:20   auth   Password compared with ==; use subtle.ConstantTimeCompare so the timing doesn't leak it

Error returns no test reaches:
  shop/order.go:47  return 0, fmt.Errorf("parse quantity %q: %w", s, err)

1 untested error path.
```

- **Untested candidates:** code matching a pattern that no test file in the same directory
  mentions by name.
- **Hints:** money in floats (`amount`, `price`, `cost`, `balance`, `fee`), `time.Now()` read
  as a calendar value without a time zone, and secrets (`password`, `token`, `secret`,
  `session`) compared with `==`. Advice only, they never fail the check.
- **Error paths:** lazytest runs your tests with coverage and lists every `return err` (bare or
  wrapped) that no test reaches. Here the example's own test only covers the happy path of
  `ParseQuantity`.

## Findings vs. "needs setup"

Methods are tested on a zero-valued receiver, since lazytest can't know your dependencies.
A handler that touches its database then panics. Those failures are listed apart and don't
fail the run. Shortened output from [lifelog](docs/real-world.md#lifelog):

```text
Needs setup (methods panicked on a zero-valued receiver, give them real dependencies):
  · internal/api  TestLazytest_Server_handleGit_BadInput/empty_request  panic: runtime error: invalid memory address or nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)
  …

0 findings, 4 tests without setup.
```

Fill in the receiver in the generated file, e.g. `handler := (&Server{db: testDB}).handleGit`.

## JSON

`--json` prints the same result for scripts and CI:

```json
{
  "hints": [
    {
      "file": "shop/order.go",
      "line": 15,
      "domain": "money",
      "message": "Price is a float64; floats can't hold cents exactly, use integer cents or a decimal type"
    }
  ],
  "errorPaths": [
    {
      "file": "shop/order.go",
      "line": 47,
      "code": "return 0, fmt.Errorf(\"parse quantity %q: %w\", s, err)"
    }
  ]
}
```

Keys: `candidates`, `hints`, `errorPaths`, `generated`, `findings`. Empty ones are left out.

## Limits

lazytest reads syntax only, it never type-checks. That keeps it fast and dependency-free, and
it's why some things slip through:

- I/O hidden behind a call to another function isn't seen, so such a func can still be
  treated as pure.
- Slices are fuzzed with one element. Nil and empty slices are only covered in validator tables.
- Pointers, maps and nested structs stay at their zero value in JSON round trips.
- "Tested" means a test file mentions the name. Error paths use real coverage.
- Constructors with params (`NewHandler(db)`) are skipped.

## More

- [How it works](docs/how-it-works.md): architecture, every detection rule, design decisions
- [Real-world runs](docs/real-world.md): what lazytest found on real projects
- [Pitch](PITCH.md): the original brief

Try it on the example yourself, from a clone of this repo:

```bash
go install ./cmd/lazytest
cd _example && lazytest --run ./...
```

## License

[MIT](LICENSE)
