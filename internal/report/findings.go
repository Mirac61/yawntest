package report

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
)

// failure is one generated test func that failed, its failed cases folded in.
type failure struct {
	file    string // shop/order.go, empty if the source is unknown
	line    int
	where   string // shop/order.go:30, or the dir if the source is unknown
	name    string // Split, or the test name
	message string
	cases   int
	rerun   string // go test command that runs the first failed case alone
}

// generated maps the failures back to their source; it may miss files written by an earlier run.
func findings(p *printer, all []run.Finding, generated []GeneratedFile) {
	real, needsSetup := splitFindings(all)
	if bugs := group(real, generated); len(bugs) > 0 {
		p.section("Findings, generated tests that fail on your code")
		writeFailures(p, paint(red, "✗"), bugs, true)
	}
	if setup := group(needsSetup, generated); len(setup) > 0 {
		p.section("Needs setup, methods that panicked on a zero-valued receiver")
		writeFailures(p, paint(gray, "·"), setup, false)
	}
}

func splitFindings(findings []run.Finding) (real, needsSetup []run.Finding) {
	for _, finding := range findings {
		if finding.NeedsSetup {
			needsSetup = append(needsSetup, finding)
		} else {
			real = append(real, finding)
		}
	}
	return real, needsSetup
}

// group folds the cases of one test func (TestYawntest_Create_BadInput/empty_request) into one failure.
func group(findings []run.Finding, generated []GeneratedFile) []failure {
	var failures []failure
	index := map[string]int{}
	for _, finding := range findings {
		testFunc, _, _ := strings.Cut(finding.Test, "/")
		key := finding.Dir + " " + testFunc
		if i, seen := index[key]; seen {
			failures[i].cases++
			continue
		}

		index[key] = len(failures)
		located := locate(finding.Dir, testFunc, generated)
		located.message, located.cases = finding.Message, 1
		located.rerun = rerunCommand(finding.Dir, finding.Test)
		failures = append(failures, located)
	}
	return failures
}

// locate finds the match a test func was generated for, the longest name winning:
// TestYawntest_Server_Create_BadInput belongs to Server.Create, not Server.
func locate(dir, testFunc string, generated []GeneratedFile) failure {
	if testFunc == "" {
		return failure{where: dir, name: "build failed"}
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(testFunc, "TestYawntest_"), "FuzzYawntest_")

	located := failure{where: dir, name: rest}
	longest := 0
	for _, file := range generated {
		if filepath.Clean(filepath.Dir(file.Source)) != filepath.Clean(dir) {
			continue
		}
		for _, test := range file.Output.Tests {
			prefix := gen.TestName(test.Match.Name)
			fits := rest == prefix || strings.HasPrefix(rest, prefix+"_")
			if fits && len(prefix) > longest {
				longest = len(prefix)
				located = failure{
					file:  file.Source,
					line:  test.Match.Line,
					where: fmt.Sprintf("%s:%d", file.Source, test.Match.Line),
					name:  test.Match.Name,
				}
			}
		}
	}
	return located
}

// The rerun command goes under the message: empty cells keep the columns, the last cell
// doesn't widen them. The empty first cell carries the same two (invisible) color codes as
// the marker and location above it, since tabwriter counts bytes, not what's visible.
func writeFailures(p *printer, marker string, failures []failure, withMessage bool) {
	blank := paint(gray, "") + paint(gray, "")
	p.table(func(table io.Writer) {
		for _, failure := range failures {
			fmt.Fprintf(table, "  %s %s\t%s", marker, paint(gray, failure.where), failure.name)
			if withMessage {
				fmt.Fprintf(table, "\t%s", failure.message)
				if failure.cases > 1 {
					fmt.Fprintf(table, " (%d cases)", failure.cases)
				}
			}
			fmt.Fprintln(table)
			if withMessage {
				fmt.Fprintf(table, "%s\t\t%s\n", blank, paint(gray, failure.rerun))
			} else {
				fmt.Fprintf(table, "%s\t%s\n", blank, paint(gray, failure.rerun))
			}
		}
	})
}

// rerunCommand runs one failed case alone: go test ./shop -run '^FuzzYawntest_Split$/^seed#1$'.
func rerunCommand(dir, test string) string {
	pkg := filepath.ToSlash(dir)
	if !filepath.IsAbs(dir) && pkg != "." && !strings.HasPrefix(pkg, "./") && !strings.HasPrefix(pkg, "../") {
		pkg = "./" + pkg
	}
	if test == "" {
		return "go test " + pkg
	}
	var parts []string
	for _, part := range strings.Split(test, "/") {
		parts = append(parts, "^"+regexp.QuoteMeta(part)+"$")
	}
	pattern := strings.ReplaceAll(strings.Join(parts, "/"), "'", `'\''`)
	return fmt.Sprintf("go test %s -run '%s'", pkg, pattern)
}
