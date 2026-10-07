package detect

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

func TestGolden(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/*.go")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			got := detectFixture(t, fixture)

			golden := strings.TrimSuffix(fixture, ".go") + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if got != string(want) {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func detectFixture(t *testing.T, path string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	var out strings.Builder
	for _, match := range File(fset, file) {
		fmt.Fprintf(&out, "%d %s %s: %s\n", match.Line, match.Name, match.Pattern, match.Reason)
		if len(match.Fields) > 0 {
			fmt.Fprintf(&out, "    fields: %s\n", formatFields(match.Fields))
		}
		if len(match.Params) > 0 || match.Results > 0 {
			fmt.Fprintf(&out, "    params: %s, results: %d\n", formatFields(match.Params), match.Results)
		}
	}
	return out.String()
}

func formatFields(fields []Field) string {
	var parts []string
	for _, field := range fields {
		typ := field.Type
		if typ == "" {
			typ = "?"
		}
		parts = append(parts, strings.TrimSpace(field.Name+" "+typ))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func TestMatchSymbol(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Add", want: "Add"},
		{name: "Task.Validate", want: "Validate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (Match{Name: test.name}).Symbol(); got != test.want {
				t.Errorf("Symbol() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidationPrefix(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "ValidateEmail", want: "Validate"},
		{name: "Validate", want: "Validate"},
		{name: "checkName", want: "Check"},
		{name: "IsValidAge", want: "IsValid"},
		{name: "Checkout", want: ""},
		{name: "Validated", want: ""},
		{name: "Save", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validationPrefix(test.name); got != test.want {
				t.Errorf("validationPrefix(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}
