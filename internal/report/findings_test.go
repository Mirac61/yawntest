package report

import (
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/run"
)

func TestFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings []run.Finding
		want     string
	}{
		{
			name: "all pass",
			want: "\nAll generated tests pass.\n",
		},
		{
			name: "failed test and failed build",
			findings: []run.Finding{
				{Dir: "api", Test: "TestLazytest_CreateTask_BadInput/invalid_json", Message: "status = 500, want 4xx"},
				{Dir: "calc", Message: "./calc.go:3:28: undefined: x"},
			},
			want: "\nFindings (generated tests that fail on the current code):\n" +
				"  ⚠ api   TestLazytest_CreateTask_BadInput/invalid_json  status = 500, want 4xx\n" +
				"  ⚠ calc  build failed                                   ./calc.go:3:28: undefined: x\n" +
				"\n2 findings.\n",
		},
		{
			name: "only missing setup",
			findings: []run.Finding{
				{Dir: "api", Test: "TestLazytest_Server_Health_BadInput/empty_request", Message: "panic: nil pointer", NeedsSetup: true},
			},
			want: "\nNeeds setup (methods panicked on a zero-valued receiver, give them real dependencies):\n" +
				"  · api  TestLazytest_Server_Health_BadInput/empty_request  panic: nil pointer\n" +
				"\n0 findings, 1 test without setup.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := Findings(&out, test.findings); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}
