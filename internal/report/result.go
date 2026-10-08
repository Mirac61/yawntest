package report

import (
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

// Result is everything one yawntest run found, before it's printed as text or JSON.
type Result struct {
	Untested   []scan.File // check, with hints
	ErrorPaths []run.ErrorPath
	Generated  []GeneratedFile
	Findings   []run.Finding // run
}

// HasProblems decides the exit code. Hints and tests that only miss setup don't count.
func (r Result) HasProblems() bool {
	real, _ := splitFindings(r.Findings)
	return scan.HasMatches(r.Untested) || len(r.ErrorPaths) > 0 || len(real) > 0
}
