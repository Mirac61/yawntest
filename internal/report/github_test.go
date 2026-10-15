package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

// sampleResult has one of every issue kind, in a GITHUB_WORKSPACE set to a temp dir.
func sampleResult(t *testing.T) Result {
	t.Helper()
	workspace := t.TempDir()
	t.Setenv("GITHUB_WORKSPACE", workspace)
	source := filepath.Join(workspace, "shop", "order.go")
	return Result{
		Untested: []scan.File{{
			Path:    source,
			Matches: []detect.Match{{Line: 30, Name: "Split", Pattern: detect.PatternPureFunc, Reason: "basic params, no I/O"}},
			Hints:   []detect.Hint{{Line: 15, Domain: "money", Message: "Price is a float64; use cents"}},
		}},
		ErrorPaths: []run.ErrorPath{{File: source, Line: 47, Func: "Parse", Code: `return 0, fmt.Errorf("parse: %w", err)`}},
		Generated: []GeneratedFile{{
			Source: source,
			Output: gen.Output{Tests: []gen.Test{{Match: detect.Match{Line: 30, Name: "Split"}}}},
		}},
		Findings: []run.Finding{{Dir: filepath.Dir(source), Test: "FuzzYawntest_Split/seed#1", Message: "parts add up to 99,\nwant 100"}},
		Warnings: []string{"skipped bad.go, it doesn't parse: 100% broken"},
	}
}

func TestGitHub(t *testing.T) {
	result := sampleResult(t)
	var out strings.Builder
	if err := GitHub(&out, result); err != nil {
		t.Fatalf("write: %v", err)
	}
	shop := filepath.ToSlash(result.Findings[0].Dir)
	// Most important first: findings, error paths, hints, candidates, yawntest's own warnings.
	want := "::error file=shop/order.go,line=30,title=yawntest finding%3A Split::parts add up to 99,%0Awant 100%0ARerun: go test " + shop + " -run '^FuzzYawntest_Split$/^seed#1$'\n" +
		"::warning file=shop/order.go,line=47,title=yawntest%3A untested error path::No test reaches this return in Parse: return 0, fmt.Errorf(\"parse: %25w\", err)\n" +
		"::warning file=shop/order.go,line=15,title=yawntest%3A money::Price is a float64; use cents\n" +
		"::notice file=shop/order.go,line=30,title=yawntest%3A untested pure-func::Split has no test yet (basic params, no I/O). `yawntest gen` writes one.\n" +
		"::warning title=yawntest::skipped bad.go, it doesn't parse: 100%25 broken\n"
	if got := out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// GitHub drops everything past 10 per level without a word, so yawntest says how many it held back.
func TestGitHubLimit(t *testing.T) {
	t.Setenv("GITHUB_WORKSPACE", "")
	var matches []detect.Match
	var hints []detect.Hint
	for i := 1; i <= 12; i++ {
		matches = append(matches, detect.Match{Line: i, Name: fmt.Sprintf("F%d", i), Pattern: detect.PatternPureFunc})
		hints = append(hints, detect.Hint{Line: i, Domain: "money"})
	}
	result := Result{Untested: []scan.File{{Path: "a.go", Matches: matches, Hints: hints}}}

	var out strings.Builder
	if err := GitHub(&out, result); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	notices, warnings := strings.Count(got, "::notice "), strings.Count(got, "::warning ")
	if notices != 10 || warnings != 10 {
		t.Errorf("%d notices and %d warnings, want 10 each (9 candidates and the overflow notice)", notices, warnings)
	}
	// 3 candidates (one gave its slot to the overflow notice) and 2 hints were held back.
	if !strings.Contains(got, "::notice title=yawntest::5 more issues not shown here") {
		t.Errorf("no overflow notice:\n%s", got)
	}
}

func TestSARIF(t *testing.T) {
	var out strings.Builder
	if err := SARIF(&out, sampleResult(t), "v1.2.3"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		`"version": "2.1.0"`,
		`"name": "yawntest"`,
		`"version": "v1.2.3"`,
		`"id": "error-path"`,
		`"id": "untested/pure-func"`,
		`"ruleId": "finding"`,
		`"level": "note"`,
		`"uri": "shop/order.go"`,
		`"uriBaseId": "%SRCROOT%"`,
		`"startLine": 47`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	// The warning about bad.go has no location, and code scanning rejects results without one.
	if strings.Contains(got, "bad.go") {
		t.Errorf("result without a location in:\n%s", got)
	}
}

// The workspace path may go through a symlink the cwd doesn't, like /var and /private/var on macOS.
func TestRepoPathThroughSymlink(t *testing.T) {
	real := t.TempDir()
	if err := os.MkdirAll(filepath.Join(real, "shop"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "workspace")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	// order.go doesn't exist: a path is still resolved as far as it goes.
	if got := repoPath(link, filepath.Join(real, "shop", "order.go")); got != "shop/order.go" {
		t.Errorf("repoPath = %q, want shop/order.go", got)
	}
	if got := repoPath("", "shop/order.go"); got != "shop/order.go" {
		t.Errorf("repoPath without a workspace = %q", got)
	}
}
