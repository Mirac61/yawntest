package report

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
)

func TestFindings(t *testing.T) {
	generated := []GeneratedFile{{
		Source: "api/tasks.go",
		Output: gen.Output{Tests: []gen.Test{
			{Match: detect.Match{Name: "Server", Line: 5}},
			{Match: detect.Match{Name: "Server.Create", Line: 12}},
			{Match: detect.Match{Name: "Split", Line: 30}},
		}},
	}}

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
			name: "cases of one test are folded into one line",
			findings: []run.Finding{
				{Dir: "api", Test: "TestYawntest_Server_Create_BadInput/empty_request", Message: "status = 500, want < 500 for bad input"},
				{Dir: "api", Test: "TestYawntest_Server_Create_BadInput/invalid_json", Message: "status = 500, want < 500 for bad input"},
				{Dir: "api", Test: "FuzzYawntest_Split/seed#5", Message: "Split(100, 3) parts add up to 99"},
			},
			want: "\nFindings, generated tests that fail on your code:\n" +
				"  ✗ api/tasks.go:12  Server.Create  status = 500, want < 500 for bad input (2 cases)\n" +
				"  ✗ api/tasks.go:30  Split          Split(100, 3) parts add up to 99\n" +
				"\n2 findings.\n",
		},
		{
			name: "unknown source and failed build",
			findings: []run.Finding{
				{Dir: "calc", Test: "FuzzYawntest_Add/seed#1", Message: "panic: overflow"},
				{Dir: "calc", Message: "./calc.go:3:28: undefined: x"},
			},
			want: "\nFindings, generated tests that fail on your code:\n" +
				"  ✗ calc  Add           panic: overflow\n" +
				"  ✗ calc  build failed  ./calc.go:3:28: undefined: x\n" +
				"\n2 findings.\n",
		},
		{
			name: "only missing setup",
			findings: []run.Finding{
				{Dir: "api", Test: "TestYawntest_Server_Create_BadInput/empty_request", Message: "panic: nil pointer", NeedsSetup: true},
			},
			want: "\nNeeds setup, methods that panicked on a zero-valued receiver:\n" +
				"  · api/tasks.go:12  Server.Create\n" +
				"\n0 findings, 1 test without setup.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := Findings(&out, test.findings, generated); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}

// Colors must not shift the columns: without its escape codes the output equals the plain one.
func TestFindingsColor(t *testing.T) {
	findings := []run.Finding{
		{Dir: "api", Test: "FuzzYawntest_Add/seed#1", Message: "panic: overflow"},
		{Dir: "calculator", Test: "FuzzYawntest_Sub/seed#1", Message: "panic: overflow"},
		{Dir: "api", Test: "TestYawntest_Get_BadInput/empty_request", NeedsSetup: true},
	}

	var plain, colored strings.Builder
	if err := Findings(&plain, findings, nil); err != nil {
		t.Fatal(err)
	}
	Color = true
	defer func() { Color = false }()
	if err := Findings(&colored, findings, nil); err != nil {
		t.Fatal(err)
	}

	stripped := regexp.MustCompile("\x1b\\[[0-9]+m").ReplaceAllString(colored.String(), "")
	if colored.String() == plain.String() || stripped != plain.String() {
		t.Errorf("colored:\n%s\nplain:\n%s", colored.String(), plain.String())
	}
}
