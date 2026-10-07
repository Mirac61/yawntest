package scan

import (
	"slices"
	"testing"
)

// testdata holds:
//   - Tested: called from a_test.go
//   - Untested: no test mentions it
//   - OnlyInLazytest: covered by a generated lazytest file
//   - Generated: lives in a "Code generated ... DO NOT EDIT." file
//   - Vendored: lives under vendor/
func TestUntestedReportsOnlyUncoveredSymbols(t *testing.T) {
	matches, err := Untested("testdata")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	var names []string
	for _, match := range matches {
		names = append(names, match.Name)
	}
	if want := []string{"Untested"}; !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestSkipDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "vendor", want: true},
		{name: "testdata", want: true},
		{name: ".git", want: true},
		{name: "_examples", want: true},
		{name: "internal", want: false},
		{name: "vendors", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := skipDir(test.name); got != test.want {
				t.Errorf("skipDir(%q) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}
