package run

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseTestEvents(t *testing.T) {
	events := `
{"Action":"run","Test":"TestLazytest_CreateTask_BadInput"}
{"Action":"run","Test":"TestLazytest_CreateTask_BadInput/invalid_json"}
{"Action":"output","Test":"TestLazytest_CreateTask_BadInput/invalid_json","Output":"=== RUN   TestLazytest_CreateTask_BadInput/invalid_json\n"}
{"Action":"output","Test":"TestLazytest_CreateTask_BadInput/invalid_json","Output":"    api_lazytest_test.go:35: status = 500, want < 500 for bad input\n"}
{"Action":"fail","Test":"TestLazytest_CreateTask_BadInput/invalid_json"}
{"Action":"fail","Test":"TestLazytest_CreateTask_BadInput"}
{"Action":"output","Test":"FuzzLazytest_Initial/seed#0","Output":"--- FAIL: FuzzLazytest_Initial/seed#0 (0.00s)\n"}
{"Action":"fail","Test":"FuzzLazytest_Initial/seed#0"}
{"Action":"output","Test":"FuzzLazytest_Initial","Output":"panic: runtime error: index out of range [0] with length 0\n"}
{"Action":"fail","Test":"FuzzLazytest_Initial"}
{"Action":"output","Test":"TestLazytest_Server_handleGit_BadInput/empty_request","Output":"    git_lazytest_test.go:21: panic: nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)\n"}
{"Action":"fail","Test":"TestLazytest_Server_handleGit_BadInput/empty_request"}
{"Action":"pass","Test":"TestLazytest_Task_JSONRoundtrip"}
{"Action":"fail"}
`
	buildFailure := `
{"ImportPath":"broken","Action":"build-output","Output":"# broken\n"}
{"ImportPath":"broken","Action":"build-output","Output":"./broken.go:3:28: cannot use \"x\" as int value\n"}
{"ImportPath":"broken","Action":"build-fail"}
{"Action":"fail","Package":"broken","FailedBuild":"broken"}
`
	tests := []struct {
		name   string
		events string
		want   []Finding
	}{
		{
			name:   "failed tests",
			events: events,
			want: []Finding{
				{Test: "TestLazytest_CreateTask_BadInput/invalid_json", Message: "status = 500, want < 500 for bad input"},
				{Test: "FuzzLazytest_Initial/seed#0", Message: "panic: runtime error: index out of range [0] with length 0"},
				{
					Test:       "TestLazytest_Server_handleGit_BadInput/empty_request",
					Message:    "panic: nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)",
					NeedsSetup: true,
				},
			},
		},
		{
			name:   "build failure",
			events: buildFailure,
			want:   []Finding{{Message: `./broken.go:3:28: cannot use "x" as int value`}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findings, err := parseTestEvents(strings.NewReader(test.events))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !slices.Equal(findings, test.want) {
				t.Errorf("findings = %+v, want %+v", findings, test.want)
			}
		})
	}
}

func TestLazytestTestsReportsFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module bug\n\ngo 1.22\n")
	writeFile(t, dir, "bug.go", "package bug\n\nfunc Initial(name string) byte { return name[0] }\n")
	writeFile(t, dir, "bug_lazytest_test.go", `package bug

import "testing"

func TestLazytest_Initial(t *testing.T) { Initial("") }

func TestLazytest_Fine(t *testing.T) {}

func TestUserWritten(t *testing.T) { t.Fatal("not run by lazytest") }
`)

	findings, err := LazytestTests([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if got, want := findings[0].Test, "TestLazytest_Initial"; got != want {
		t.Errorf("Test = %q, want %q", got, want)
	}
	if !strings.HasPrefix(findings[0].Message, "panic:") {
		t.Errorf("Message = %q, want a panic", findings[0].Message)
	}
}

func TestLazytestTestsReportsBuildFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module broken\n\ngo 1.22\n")
	writeFile(t, dir, "broken.go", "package broken\n\nfunc Broken() int { return \"x\" }\n")

	findings, err := LazytestTests([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(findings) != 1 || findings[0].Test != "" {
		t.Fatalf("findings = %+v, want one build failure", findings)
	}
	if !strings.Contains(findings[0].Message, "broken.go") {
		t.Errorf("Message = %q, want the compiler error", findings[0].Message)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
