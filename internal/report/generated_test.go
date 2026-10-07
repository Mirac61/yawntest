package report

import (
	"strings"
	"testing"

	"github.com/Mirac61/lazytest/internal/detect"
	"github.com/Mirac61/lazytest/internal/gen"
)

func TestGenerated(t *testing.T) {
	task := detect.Match{Name: "Task", Pattern: detect.PatternJSONRoundtrip}
	add := detect.Match{Name: "Add", Pattern: detect.PatternPureFunc}
	handler := detect.Match{Name: "CreateTask", Pattern: detect.PatternHTTPHandler}

	tests := []struct {
		name  string
		files []GeneratedFile
		want  string
	}{
		{
			name: "nothing found",
			want: "Nothing to do.\n",
		},
		{
			name: "written file with a skipped candidate",
			files: []GeneratedFile{{
				Source:   "api/tasks.go",
				TestPath: "api/tasks_lazytest_test.go",
				Written:  true,
				Output: gen.Output{
					Tests: []gen.Test{
						{Match: task, Summary: "1 test"},
						{Match: add, Summary: "fuzz, 5 seeds"},
					},
					Skipped: []detect.Match{handler},
				},
			}},
			want: "api/tasks.go\n" +
				"  ✓ Task        json-roundtrip  1 test\n" +
				"  ✓ Add         pure-func       fuzz, 5 seeds\n" +
				"  · CreateTask  http-handler    no generator yet\n" +
				"\nWrote 1 file with 2 tests. 1 candidate without a generator yet.\n",
		},
		{
			name: "existing file without --force",
			files: []GeneratedFile{{
				Source:   "a.go",
				TestPath: "a_lazytest_test.go",
				Output:   gen.Output{Tests: []gen.Test{{Match: add, Summary: "fuzz, 5 seeds"}}},
			}},
			want: "a.go\n" +
				"  ! Add  pure-func  a_lazytest_test.go exists, rerun with --force\n" +
				"\nWrote 0 files with 0 tests. 0 candidates without a generator yet.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := Generated(&out, test.files); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}
