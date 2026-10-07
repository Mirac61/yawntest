package report

import (
	"github.com/Mirac61/lazytest/internal/run"
	"github.com/Mirac61/lazytest/internal/scan"
)

// Result is everything one lazytest run found, before it's printed as text or JSON.
type Result struct {
	Untested   []scan.File // --check, with hints
	ErrorPaths []run.ErrorPath
	Generated  []GeneratedFile
	Findings   []run.Finding // --run
}

// HasProblems decides the exit code. Hints are advice and don't count.
func (r Result) HasProblems() bool {
	return scan.HasMatches(r.Untested) || len(r.ErrorPaths) > 0 || len(r.Findings) > 0
}
