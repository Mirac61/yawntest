package report

import (
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

func TestHasProblems(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   bool
	}{
		{name: "nothing", result: Result{}, want: false},
		{name: "untested candidate", result: Result{Untested: []scan.File{{Matches: []detect.Match{{Name: "Add"}}}}}, want: true},
		{name: "only hints", result: Result{Untested: []scan.File{{Hints: []detect.Hint{{Domain: "money"}}}}}, want: false},
		{name: "error path", result: Result{ErrorPaths: []run.ErrorPath{{Line: 1}}}, want: true},
		{name: "finding", result: Result{Findings: []run.Finding{{Test: "TestYawntest_Add"}}}, want: true},
		{name: "only missing setup", result: Result{Findings: []run.Finding{{NeedsSetup: true}}}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.result.HasProblems(); got != test.want {
				t.Errorf("HasProblems() = %v, want %v", got, test.want)
			}
		})
	}
}
