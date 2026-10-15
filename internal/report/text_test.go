package report

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

var (
	check = View{Command: "check"}
	genV  = View{Command: "gen"}
	runV  = View{Command: "run"}
)

// generatedTasks is what gen wrote for api/tasks.go in the run cases.
var generatedTasks = []GeneratedFile{{
	Source:   "api/tasks.go",
	TestPath: "api/tasks_yawntest_test.go",
	Written:  true,
	Output: gen.Output{Tests: []gen.Test{
		{Match: detect.Match{Name: "Server", Line: 5, Pattern: detect.PatternJSONRoundtrip}, Summary: "1 test"},
		{Match: detect.Match{Name: "Server.Create", Line: 12, Pattern: detect.PatternHTTPHandler}, Summary: "3 cases"},
		{Match: detect.Match{Name: "Split", Line: 30, Pattern: detect.PatternPureFunc}, Summary: "fuzz, 9 seeds"},
	}},
}}

func TestText(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		view   View
		want   string
	}{
		{
			name: "check finds nothing",
			view: check,
			want: "Nothing untested\n",
		},
		{
			name: "check: one row per declaration, path:line first",
			view: check,
			result: Result{
				Untested: []scan.File{
					{Path: "shop/status.go", Matches: []detect.Match{
						{Line: 30, Name: "ParseStatus", Pattern: detect.PatternPureFunc, Reason: "basic params, no I/O"},
						{Line: 30, Name: "ParseStatus", Pattern: detect.PatternEnumRoundtrip, Reason: "Status has String and consts"},
					}},
					{Path: "shop/order.go",
						Matches: []detect.Match{{Line: 11, Name: "Order", Pattern: detect.PatternJSONRoundtrip, Reason: "has json tags"}},
						Hints:   []detect.Hint{{Line: 15, Domain: "money", Message: "Price is a float64"}},
					},
				},
				ErrorPaths: []run.ErrorPath{{File: "shop/order.go", Line: 47, Func: "ParseQuantity", Code: "return 0, err"}},
				NoTests:    []string{"api"},
			},
			want: "Untested\n" +
				"  shop/status.go:30  ParseStatus  pure-func, enum-roundtrip  Status has String and consts\n" +
				"  shop/order.go:11   Order        json-roundtrip             has json tags\n" +
				"\nHints\n" +
				"  shop/order.go:15  money  Price is a float64\n" +
				"\nError paths no test reaches\n" +
				"  shop/order.go:47  ParseQuantity  return 0, err\n" +
				"\nNo tests at all, so no error paths listed\n" +
				"  api\n" +
				"\n3 untested candidates · 1 hint · 1 untested error path  → yawntest gen writes the tests\n",
		},
		{
			name:   "check --static says nothing about error paths",
			view:   View{Command: "check", Static: true},
			result: Result{Untested: []scan.File{{Path: "a.go", Hints: []detect.Hint{{Line: 1, Domain: "auth", Message: "token =="}}}}},
			want: "Hints\n" +
				"  a.go:1  auth  token ==\n" +
				"\nNothing untested · 1 hint\n",
		},
		{
			name: "gen with a skipped candidate and an existing file",
			view: genV,
			result: Result{Generated: []GeneratedFile{
				{
					Source: "api/tasks.go", TestPath: "api/tasks_yawntest_test.go", Written: true,
					Output: gen.Output{
						Tests:   []gen.Test{{Match: detect.Match{Name: "Add", Line: 3, Pattern: detect.PatternPureFunc}, Summary: "fuzz, 5 seeds"}},
						Skipped: []detect.Match{{Name: "NewHandler", Line: 9, Pattern: detect.PatternHTTPHandler}},
					},
				},
				{
					Source: "a.go", TestPath: "a_yawntest_test.go",
					Output: gen.Output{Tests: []gen.Test{{Match: detect.Match{Name: "Sub", Line: 1, Pattern: detect.PatternPureFunc}}}},
				},
			}},
			want: "Generated\n" +
				"  ✓ api/tasks.go:3  Add         pure-func     fuzz, 5 seeds\n" +
				"  · api/tasks.go:9  NewHandler  http-handler  skipped, needs inputs yawntest can't build\n" +
				"  ! a.go:1          Sub         pure-func     a_yawntest_test.go exists, rerun with --force\n" +
				"\nWrote 1 file with 1 test, skipped 1 candidate  → yawntest run runs them\n",
		},
		{
			name: "gen with nothing to do",
			view: genV,
			want: "Nothing to do\n",
		},
		{
			name:   "run: cases of one test fold into one line, with a command to rerun one",
			view:   runV,
			result: Result{Generated: generatedTasks, Findings: []run.Finding{
				{Dir: "api", Test: "TestYawntest_Server_Create_BadInput/empty_request", Message: "status = 500, want < 500 for bad input"},
				{Dir: "api", Test: "TestYawntest_Server_Create_BadInput/invalid_json", Message: "status = 500, want < 500 for bad input"},
				{Dir: "api", Test: "FuzzYawntest_Split/seed#5", Message: "Split(100, 3) parts add up to 99"},
				{Dir: "calc", Message: "./calc.go:3:28: undefined: x"},
				{Dir: "api", Test: "TestYawntest_Server_Get_BadInput/empty_request", Message: "panic: nil", NeedsSetup: true},
			}},
			want: "Generated\n" +
				"  ✓ api/tasks.go:5   Server         json-roundtrip  1 test\n" +
				"  ✓ api/tasks.go:12  Server.Create  http-handler    3 cases\n" +
				"  ✓ api/tasks.go:30  Split          pure-func       fuzz, 9 seeds\n" +
				"\nFindings, generated tests that fail on your code\n" +
				"  ✗ api/tasks.go:12  Server.Create  status = 500, want < 500 for bad input (2 cases)\n" +
				"                                    go test ./api -run '^TestYawntest_Server_Create_BadInput$/^empty_request$'\n" +
				"  ✗ api/tasks.go:30  Split          Split(100, 3) parts add up to 99\n" +
				"                                    go test ./api -run '^FuzzYawntest_Split$/^seed#5$'\n" +
				"  ✗ calc             build failed   ./calc.go:3:28: undefined: x\n" +
				"                                    go test ./calc\n" +
				"\nNeeds setup, methods that panicked on a zero-valued receiver\n" +
				"  · api/tasks.go:5  Server\n" +
				"                    go test ./api -run '^TestYawntest_Server_Get_BadInput$/^empty_request$'\n" +
				"\nWrote 1 file with 3 tests · 3 findings · 1 test without setup\n",
		},
		{
			name:   "run where everything passes",
			view:   runV,
			result: Result{Generated: generatedTasks[:1]},
			want: "Generated\n" +
				"  ✓ api/tasks.go:5   Server         json-roundtrip  1 test\n" +
				"  ✓ api/tasks.go:12  Server.Create  http-handler    3 cases\n" +
				"  ✓ api/tasks.go:30  Split          pure-func       fuzz, 9 seeds\n" +
				"\nWrote 1 file with 3 tests · all generated tests pass\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := Text(&out, test.result, test.view); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}

func TestRerunCommand(t *testing.T) {
	tests := []struct{ dir, test, want string }{
		{dir: "shop", test: "FuzzYawntest_Split/seed#1", want: "go test ./shop -run '^FuzzYawntest_Split$/^seed#1$'"},
		{dir: ".", test: "TestYawntest_Add", want: "go test . -run '^TestYawntest_Add$'"},
		{dir: "/abs/api", test: "", want: "go test /abs/api"},
		{dir: "./x", test: "TestYawntest_A/it's", want: `go test ./x -run '^TestYawntest_A$/^it'\''s$'`},
		{dir: "x", test: "TestYawntest_A/a.b", want: `go test ./x -run '^TestYawntest_A$/^a\.b$'`},
	}
	for _, test := range tests {
		if got := rerunCommand(test.dir, test.test); got != test.want {
			t.Errorf("rerunCommand(%q, %q) = %s, want %s", test.dir, test.test, got, test.want)
		}
	}
}

// Colors must not shift the columns: without its escape codes the output equals the plain one.
func TestTextColor(t *testing.T) {
	result := Result{
		Untested: []scan.File{{Path: "a.go",
			Matches: []detect.Match{{Line: 1, Name: "A", Pattern: detect.PatternPureFunc}, {Line: 20, Name: "LongerName", Pattern: detect.PatternValidation}},
			Hints:   []detect.Hint{{Line: 3, Domain: "money"}, {Line: 4, Domain: "dates"}},
		}},
		Generated: generatedTasks,
		Findings: []run.Finding{
			{Dir: "api", Test: "FuzzYawntest_Split/seed#1", Message: "panic: overflow"},
			{Dir: "calculator", Test: "FuzzYawntest_Sub/seed#1", Message: "panic: overflow"},
		},
	}
	for _, view := range []View{check, runV} {
		var plain, colored strings.Builder
		if err := Text(&plain, result, view); err != nil {
			t.Fatal(err)
		}
		Color = true
		err := Text(&colored, result, view)
		Color = false
		if err != nil {
			t.Fatal(err)
		}

		stripped := regexp.MustCompile("\x1b\\[[0-9]+m").ReplaceAllString(colored.String(), "")
		if colored.String() == plain.String() || stripped != plain.String() {
			t.Errorf("%s colored:\n%s\nplain:\n%s", view.Command, colored.String(), plain.String())
		}
	}
}
