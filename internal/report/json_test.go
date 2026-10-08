package report

import (
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name:   "empty",
			result: Result{},
			want:   "{}\n",
		},
		{
			name: "check",
			result: Result{
				Untested: []scan.File{{
					Path:    "shop/item.go",
					Matches: []detect.Match{{Line: 3, Name: "Item", Pattern: detect.PatternJSONRoundtrip, Reason: "has json tags"}},
					Hints:   []detect.Hint{{Line: 5, Domain: "money", Message: "UnitPrice is a float64"}},
				}},
				ErrorPaths: []run.ErrorPath{{File: "shop/item.go", Line: 9, Code: "return err"}},
			},
			want: `{
  "candidates": [
    {
      "file": "shop/item.go",
      "line": 3,
      "name": "Item",
      "pattern": "json-roundtrip",
      "reason": "has json tags"
    }
  ],
  "hints": [
    {
      "file": "shop/item.go",
      "line": 5,
      "domain": "money",
      "message": "UnitPrice is a float64"
    }
  ],
  "errorPaths": [
    {
      "file": "shop/item.go",
      "line": 9,
      "code": "return err"
    }
  ]
}
`,
		},
		{
			name: "generate and run",
			result: Result{
				Generated: []GeneratedFile{{
					Source:   "calc.go",
					TestPath: "calc_yawntest_test.go",
					Written:  true,
					Output: gen.Output{
						Tests:   []gen.Test{{Match: detect.Match{Name: "Add", Pattern: detect.PatternPureFunc}, Summary: "fuzz, 5 seeds"}},
						Skipped: []detect.Match{{Name: "NewHandler"}},
					},
				}},
				Findings: []run.Finding{{Dir: ".", Test: "FuzzYawntest_Add/seed#3", Message: "panic: overflow"}},
			},
			want: `{
  "generated": [
    {
      "source": "calc.go",
      "testFile": "calc_yawntest_test.go",
      "written": true,
      "tests": [
        {
          "name": "Add",
          "pattern": "pure-func",
          "summary": "fuzz, 5 seeds"
        }
      ],
      "skipped": [
        "NewHandler"
      ]
    }
  ],
  "findings": [
    {
      "dir": ".",
      "test": "FuzzYawntest_Add/seed#3",
      "message": "panic: overflow"
    }
  ]
}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := JSON(&out, test.result); err != nil {
				t.Fatalf("write json: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}
