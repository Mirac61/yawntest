//go:build realworld

// Package realworld runs yawntest on real projects at pinned commits and keeps a summary of
// what it found as a golden file per project. A rule change shows its effect on real code in
// the diff of testdata/, and docs/real-world.md has the hand-checked verdict on each finding.
//
//	go test -tags realworld ./internal/realworld            # compare
//	go test -tags realworld ./internal/realworld -update    # rewrite the summaries
//
// Clones are cached in $YAWNTEST_CORPUS, or the user cache dir. A run takes a few minutes:
// every project's own tests run, with coverage.
package realworld

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden")

type project struct {
	name, repo, commit string
}

// Popular, varied, and testable without a database or network.
var corpus = []project{
	{"afero", "spf13/afero", "eb6a92826ea568e3f40ab91fcbda8d2b38b61d3e"},
	{"chi", "go-chi/chi", "3a33ba578e0dd93444e0ff386cc26e665d92e31e"},
	{"cli", "urfave/cli", "5a02496198b17cadfe72fe8697618e63d27ed9fb"},
	{"cobra", "spf13/cobra", "adbc8813901bba65827259daa8e22ff94ec1f30e"},
	{"decimal", "shopspring/decimal", "e799c20820a0ac572bb26833847420fdeba007df"},
	{"echo", "labstack/echo", "8fd0e184dd390ffdf082f5eda247c3ab41d5d290"},
	{"gin", "gin-gonic/gin", "0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646"},
	{"gjson", "tidwall/gjson", "690362d6edf4bcbde1e9f54d552d3814f1cd5bcb"},
	{"go-humanize", "dustin/go-humanize", "f5b54ae1392365a578bb43ab728e3fcdc291f0cd"},
	{"go-toml", "pelletier/go-toml", "686c980c47581bf19b6119b60e9a59f2aed04170"},
	{"logrus", "sirupsen/logrus", "6ef0748d1177dadaeee9aa79ffef604570f487d7"},
	{"mux", "gorilla/mux", "db9d1d0073d27a0a2d9a8c1bc52aa0af4374d265"},
	{"toml", "BurntSushi/toml", "d733fc535e4a9f3c454e421a87b90ade5d49bf31"},
	{"uuid", "google/uuid", "2d3c2a9cc518326daf99a383f07c4d3c44317e4d"},
	{"validator", "go-playground/validator", "3b54840be2c8bda33a270f068f3d781981dfa662"},
}

var binary string

func TestMain(m *testing.M) {
	flag.Parse()
	dir, err := os.MkdirTemp("", "yawntest-realworld-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "yawntest")
	if output, err := exec.Command("go", "build", "-o", binary, "../../cmd/yawntest").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build yawntest: %v\n%s", err, output)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestCorpus(t *testing.T) {
	cache := os.Getenv("YAWNTEST_CORPUS")
	if cache == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		cache = filepath.Join(userCache, "yawntest-corpus")
	}

	for _, p := range corpus {
		t.Run(p.name, func(t *testing.T) {
			dir := checkout(t, cache, p)
			start := time.Now()
			check := yawntest(t, dir, "check", "--json", "./...")
			checkTime := time.Since(start)
			start = time.Now()
			// --all: the invariants must hold for tested code too, and real projects are well
			// tested. Every finding here is either a bug or a false positive of yawntest.
			run := yawntest(t, dir, "run", "--all", "--json", "./...")
			t.Logf("%s: check %s, run %s", p.name, checkTime.Round(time.Second), time.Since(start).Round(time.Second))

			got := summarize(p, check, run)
			golden := filepath.Join("testdata", p.name+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("summary changed (rerun with -update if that's intended):\n%s", got)
			}
		})
	}
}

// checkout clones p once into the cache and resets it to the pinned commit, without what an
// earlier run wrote.
func checkout(t *testing.T, cache string, p project) string {
	t.Helper()
	dir := filepath.Join(cache, p.name)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		git(t, "", "clone", "--quiet", "--filter=blob:none", "https://github.com/"+p.repo, dir)
	}
	if err := exec.Command("git", "-C", dir, "cat-file", "-e", p.commit+"^{commit}").Run(); err != nil {
		git(t, dir, "fetch", "--quiet", "origin", p.commit)
	}
	git(t, dir, "checkout", "--quiet", "--force", p.commit)
	git(t, dir, "clean", "-fdxq")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

type result struct {
	Candidates []struct {
		File, Name, Pattern string
		Line                int
	}
	Hints []struct {
		File, Domain, Message string
		Line                  int
	}
	ErrorPaths []struct {
		File, Func, Code string
		Line             int
	}
	NoTests   []string
	Generated []struct {
		Written bool
		Tests   []struct{ Name string }
		Skipped []string
	}
	Findings []struct {
		File, Name, Dir, Test, Message string
		Line                           int
		NeedsSetup                     bool
	}
	Warnings []string
}

func yawntest(t *testing.T, dir string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("yawntest %v: %v\n%s", args, err, stderr.String())
		}
	}
	var r result
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("yawntest %v: bad JSON: %v\n%s", args, err, stdout.String())
	}
	return r
}

// summarize keeps what a rule change would move: counts, and every finding with its place.
func summarize(p project, check, run result) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s at %s\n\n", p.repo, p.commit[:12])

	patterns := map[string]int{}
	for _, c := range check.Candidates {
		patterns[c.Pattern]++
	}
	domains := map[string]int{}
	for _, h := range check.Hints {
		domains[h.Domain]++
	}
	fmt.Fprintf(&out, "check: %d candidates%s, %d hints%s, %d error paths, %d packages without tests\n",
		len(check.Candidates), counts(patterns), len(check.Hints), counts(domains), len(check.ErrorPaths), len(check.NoTests))
	for _, h := range check.Hints {
		fmt.Fprintf(&out, "  hint %s:%d %s: %s\n", h.File, h.Line, h.Domain, h.Message)
	}

	tests, skipped := 0, 0
	for _, g := range run.Generated {
		if g.Written {
			tests += len(g.Tests)
		}
		skipped += len(g.Skipped)
	}
	// Like the text report: the failed cases of one test func are one finding.
	type key struct{ dir, test string }
	seen := map[key]bool{}
	var findings []string
	setup := 0
	for _, f := range run.Findings {
		testFunc, _, _ := strings.Cut(f.Test, "/")
		if seen[key{f.Dir, testFunc}] {
			continue
		}
		seen[key{f.Dir, testFunc}] = true
		if f.NeedsSetup {
			setup++
			continue
		}
		where := f.Dir
		if f.File != "" {
			where = fmt.Sprintf("%s:%d %s", f.File, f.Line, f.Name)
		}
		findings = append(findings, fmt.Sprintf("  finding %s %s: %s\n", where, f.Test, truncate(f.Message, 160)))
	}
	fmt.Fprintf(&out, "run --all: %d tests in %d files, %d skipped, %d findings, %d need setup\n",
		tests, len(run.Generated), skipped, len(findings), setup)
	for _, finding := range findings {
		out.WriteString(finding)
	}
	for _, w := range append(check.Warnings, run.Warnings...) {
		fmt.Fprintf(&out, "  warning: %s\n", truncate(w, 160))
	}
	return out.String()
}

func counts(m map[string]int) string {
	if len(m) == 0 {
		return ""
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
