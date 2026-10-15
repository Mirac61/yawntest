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
{"Action":"run","Test":"TestYawntest_CreateTask_BadInput"}
{"Action":"run","Test":"TestYawntest_CreateTask_BadInput/invalid_json"}
{"Action":"output","Test":"TestYawntest_CreateTask_BadInput/invalid_json","Output":"=== RUN   TestYawntest_CreateTask_BadInput/invalid_json\n"}
{"Action":"output","Test":"TestYawntest_CreateTask_BadInput/invalid_json","Output":"    api_yawntest_test.go:35: status = 500, want < 500 for bad input\n"}
{"Action":"fail","Test":"TestYawntest_CreateTask_BadInput/invalid_json"}
{"Action":"fail","Test":"TestYawntest_CreateTask_BadInput"}
{"Action":"output","Test":"FuzzYawntest_Initial/seed#0","Output":"--- FAIL: FuzzYawntest_Initial/seed#0 (0.00s)\n"}
{"Action":"fail","Test":"FuzzYawntest_Initial/seed#0"}
{"Action":"output","Test":"FuzzYawntest_Initial","Output":"panic: runtime error: index out of range [0] with length 0\n"}
{"Action":"fail","Test":"FuzzYawntest_Initial"}
{"Action":"output","Test":"TestYawntest_Server_handleGit_BadInput/empty_request","Output":"    git_yawntest_test.go:21: panic: nil pointer dereference (the Server is zero-valued; give it real dependencies if it needs them)\n"}
{"Action":"fail","Test":"TestYawntest_Server_handleGit_BadInput/empty_request"}
{"Action":"pass","Test":"TestYawntest_Task_JSONRoundtrip"}
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
				{Test: "TestYawntest_CreateTask_BadInput/invalid_json", Message: "status = 500, want < 500 for bad input"},
				{Test: "FuzzYawntest_Initial/seed#0", Message: "panic: runtime error: index out of range [0] with length 0"},
				{
					Test:       "TestYawntest_Server_handleGit_BadInput/empty_request",
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

func TestYawntestTestsReportsFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module bug\n\ngo 1.22\n")
	writeFile(t, dir, "bug.go", "package bug\n\nfunc Initial(name string) byte { return name[0] }\n")
	writeFile(t, dir, "bug_yawntest_test.go", `package bug

import "testing"

func TestYawntest_Initial(t *testing.T) { Initial("") }

func TestYawntest_Fine(t *testing.T) {}

func TestUserWritten(t *testing.T) { t.Fatal("not run by yawntest") }
`)

	findings, err := YawntestTests([]string{dir}, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if got, want := findings[0].Test, "TestYawntest_Initial"; got != want {
		t.Errorf("Test = %q, want %q", got, want)
	}
	if !strings.HasPrefix(findings[0].Message, "panic:") {
		t.Errorf("Message = %q, want a panic", findings[0].Message)
	}
}

func TestYawntestTestsReportsBuildFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module broken\n\ngo 1.22\n")
	writeFile(t, dir, "broken.go", "package broken\n\nfunc Broken() int { return \"x\" }\n")

	findings, err := YawntestTests([]string{dir}, nil)
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

// A timeout ends the test binary: go test reports the package as failed, not the test that
// was running. The test that started and never finished is the finding.
func TestParseTestEventsTimeout(t *testing.T) {
	events := `
{"Action":"run","Test":"FuzzYawntest_Pow"}
{"Action":"run","Test":"FuzzYawntest_Pow/seed#0"}
{"Action":"pass","Test":"FuzzYawntest_Pow/seed#0"}
{"Action":"run","Test":"FuzzYawntest_Pow/seed#3"}
{"Action":"output","Test":"FuzzYawntest_Pow/seed#3","Output":"panic: test timed out after 2m0s\n"}
{"Action":"output","Test":"FuzzYawntest_Pow/seed#3","Output":"goroutine 65 [running]:\n"}
{"Action":"run","Test":"TestYawntest_Other"}
{"Action":"pass","Test":"TestYawntest_Other"}
{"Action":"fail","Elapsed":120.02}
`
	findings, err := parseTestEvents(strings.NewReader(events))
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{{Test: "FuzzYawntest_Pow/seed#3", Message: "still running after 2m0s: this input makes it hang or take very long"}}
	if !slices.Equal(findings, want) {
		t.Errorf("findings = %+v, want %+v", findings, want)
	}
}

func TestTestsFailed(t *testing.T) {
	crash := "--- FAIL: TestFailGenFish (0.00s)\npanic: runtime error: invalid memory address or nil pointer dereference [recovered]\n\tpanic: ...\nFAIL\tgithub.com/spf13/cobra\t0.4s\n"
	if got, want := testsFailed(crash, nil).Error(), "tests crash in TestFailGenFish: runtime error: invalid memory address or nil pointer dereference [recovered]"; got != want {
		t.Errorf("crash: %q, want %q", got, want)
	}
	build := "# example.com/x\n./x_test.go:3:2: undefined: y\nFAIL\texample.com/x [build failed]\n"
	if got, want := testsFailed(build, nil).Error(), "tests don't build: ./x_test.go:3:2: undefined: y"; got != want {
		t.Errorf("build: %q, want %q", got, want)
	}
}

func TestParseTestEventsBrokenOutput(t *testing.T) {
	if _, err := parseTestEvents(strings.NewReader(`{"Action":`)); err == nil {
		t.Error("broken go test -json output must be an error")
	}
}
