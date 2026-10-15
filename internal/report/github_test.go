package report

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

func TestGitHub(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("GITHUB_WORKSPACE", workspace)
	source := filepath.Join(workspace, "shop", "order.go")

	result := Result{
		Untested: []scan.File{{
			Path:    source,
			Matches: []detect.Match{{Line: 30, Name: "Split", Pattern: detect.PatternPureFunc, Reason: "basic params, no I/O"}},
			Hints:   []detect.Hint{{Line: 15, Domain: "money", Message: "Price is a float64; use cents"}},
		}},
		ErrorPaths: []run.ErrorPath{{File: source, Line: 47, Code: `return 0, fmt.Errorf("parse: %w", err)`}},
		Generated: []GeneratedFile{{
			Source: source,
			Output: gen.Output{Tests: []gen.Test{{Match: detect.Match{Line: 30, Name: "Split"}}}},
		}},
		Findings: []run.Finding{{Dir: filepath.Dir(source), Test: "FuzzYawntest_Split/seed#1", Message: "parts add up to 99,\nwant 100"}},
		Warnings: []string{"skipped bad.go, it doesn't parse: 100% broken"},
	}

	var out strings.Builder
	if err := GitHub(&out, result); err != nil {
		t.Fatalf("write: %v", err)
	}
	want := "::notice file=shop/order.go,line=30,title=yawntest%3A untested pure-func::Split has no test yet (basic params, no I/O). `yawntest gen` writes one.\n" +
		"::warning file=shop/order.go,line=15,title=yawntest%3A money::Price is a float64; use cents\n" +
		"::warning file=shop/order.go,line=47,title=yawntest%3A untested error path::No test reaches this return: return 0, fmt.Errorf(\"parse: %25w\", err)\n" +
		"::error file=shop/order.go,line=30,title=yawntest finding%3A Split::parts add up to 99,%0Awant 100\n" +
		"::warning title=yawntest::skipped bad.go, it doesn't parse: 100%25 broken\n"
	if got := out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
