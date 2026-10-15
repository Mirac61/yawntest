package report

import (
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

// Result is everything one yawntest run found, before it's printed as text or JSON.
type Result struct {
	Untested   []scan.File // check, with hints
	ErrorPaths []run.ErrorPath
	NoTests    []string // check: dirs with error returns but no tests at all
	Generated  []GeneratedFile
	Findings   []run.Finding // run
	Warnings   []string      // things yawntest skipped, e.g. a file that doesn't parse
}

// HasProblems decides the exit code. Hints and tests that only miss setup don't count.
func (r Result) HasProblems() bool {
	real, _ := splitFindings(r.Findings)
	return scan.HasMatches(r.Untested) || len(r.ErrorPaths) > 0 || len(real) > 0
}
