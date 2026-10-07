package report

import (
	"strings"
	"testing"

	"github.com/Mirac61/lazytest/internal/run"
)

func TestErrorPaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []run.ErrorPath
		want  string
	}{
		{
			name: "all reached",
			want: "\nEvery error return is reached by a test.\n",
		},
		{
			name: "two paths",
			paths: []run.ErrorPath{
				{File: "calc/parse.go", Line: 11, Code: `return 0, fmt.Errorf("parse: %w", err)`},
				{File: "calc/load.go", Line: 7, Code: "return err"},
			},
			want: "\nError returns no test reaches:\n" +
				"  calc/parse.go:11  return 0, fmt.Errorf(\"parse: %w\", err)\n" +
				"  calc/load.go:7    return err\n" +
				"\n2 untested error paths.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := ErrorPaths(&out, test.paths); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}
