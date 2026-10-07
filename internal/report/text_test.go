package report

import (
	"strings"
	"testing"

	"github.com/Mirac61/lazytest/internal/detect"
	"github.com/Mirac61/lazytest/internal/scan"
)

func TestText(t *testing.T) {
	tests := []struct {
		name  string
		files []scan.File
		want  string
	}{
		{
			name: "nothing found",
			want: "Nothing to do.\n",
		},
		{
			name: "one match",
			files: []scan.File{
				{Path: "api/tasks.go", Matches: []detect.Match{
					{Line: 12, Name: "CreateTask", Pattern: detect.PatternHTTPHandler, Reason: "handler signature"},
				}},
			},
			want: "api/tasks.go\n" +
				"  CreateTask  http-handler  line 12  handler signature\n" +
				"\n1 untested candidate in 1 file.\n",
		},
		{
			name: "groups by file and aligns columns",
			files: []scan.File{
				{Path: "a.go", Matches: []detect.Match{
					{Line: 3, Name: "Task", Pattern: detect.PatternJSONRoundtrip, Reason: "has json tags"},
					{Line: 10, Name: "Add", Pattern: detect.PatternPureFunc, Reason: "basic params, no I/O"},
				}},
				{Path: "b.go", Matches: []detect.Match{
					{Line: 1, Name: "Check", Pattern: detect.PatternValidation, Reason: "name Check*, returns bool"},
				}},
			},
			want: "a.go\n" +
				"  Task  json-roundtrip  line 3   has json tags\n" +
				"  Add   pure-func       line 10  basic params, no I/O\n" +
				"b.go\n" +
				"  Check  validation  line 1  name Check*, returns bool\n" +
				"\n3 untested candidates in 2 files.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := Text(&out, test.files); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}
