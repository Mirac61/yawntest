package report

import (
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/scan"
)

func TestHints(t *testing.T) {
	tests := []struct {
		name  string
		files []scan.File
		want  string
	}{
		{
			name:  "no hints",
			files: []scan.File{{Path: "a.go"}},
			want:  "",
		},
		{
			name: "hints from two files",
			files: []scan.File{
				{Path: "shop/item.go", Hints: []detect.Hint{{Line: 5, Domain: "money", Message: "UnitPrice is a float64"}}},
				{Path: "auth/login.go", Hints: []detect.Hint{{Line: 10, Domain: "auth", Message: "Password compared with =="}}},
			},
			want: "\nHints:\n" +
				"  shop/item.go:5    money  UnitPrice is a float64\n" +
				"  auth/login.go:10  auth   Password compared with ==\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := Hints(&out, test.files); err != nil {
				t.Fatalf("write report: %v", err)
			}
			if got := out.String(); got != test.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, test.want)
			}
		})
	}
}
