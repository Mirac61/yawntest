package report

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
)

// failure is one generated test func that failed, its failed cases folded in.
type failure struct {
	where   string // shop/order.go:30, or the dir if the source is unknown
	name    string // Split, or the test name
	message string
	cases   int
}

// generated maps the failures back to their source; it may miss files written by an earlier run.
func Findings(w io.Writer, findings []run.Finding, generated []GeneratedFile) error {
	real, needsSetup := splitFindings(findings)
	if len(real) == 0 && len(needsSetup) == 0 {
		_, err := fmt.Fprintln(w, "\nAll generated tests pass.")
		return err
	}

	bugs, setup := group(real, generated), group(needsSetup, generated)
	if len(bugs) > 0 {
		fmt.Fprintln(w, "\nFindings, generated tests that fail on your code:")
		if err := writeFailures(w, paint(red, "✗"), bugs, true); err != nil {
			return err
		}
	}
	if len(setup) > 0 {
		fmt.Fprintln(w, "\nNeeds setup, methods that panicked on a zero-valued receiver:")
		if err := writeFailures(w, paint(gray, "·"), setup, false); err != nil {
			return err
		}
	}

	summary := plural(len(bugs), "finding")
	if len(setup) > 0 {
		summary += fmt.Sprintf(", %s without setup", plural(len(setup), "test"))
	}
	_, err := fmt.Fprintf(w, "\n%s.\n", summary)
	return err
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
		where, name := locate(finding.Dir, testFunc, generated)
		failures = append(failures, failure{where: where, name: name, message: finding.Message, cases: 1})
	}
	return failures
}

// locate finds the match a test func was generated for, the longest name winning:
// TestYawntest_Server_Create_BadInput belongs to Server.Create, not Server.
func locate(dir, testFunc string, generated []GeneratedFile) (where, name string) {
	if testFunc == "" {
		return dir, "build failed"
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(testFunc, "TestYawntest_"), "FuzzYawntest_")

	where, name = dir, rest
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
				where, name = fmt.Sprintf("%s:%d", file.Source, test.Match.Line), test.Match.Name
			}
		}
	}
	return where, name
}

func writeFailures(w io.Writer, marker string, failures []failure, withMessage bool) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, failure := range failures {
		fmt.Fprintf(table, "  %s %s\t%s", marker, paint(gray, failure.where), failure.name)
		if withMessage {
			fmt.Fprintf(table, "\t%s", failure.message)
			if failure.cases > 1 {
				fmt.Fprintf(table, " (%d cases)", failure.cases)
			}
		}
		fmt.Fprintln(table)
	}
	return table.Flush()
}
