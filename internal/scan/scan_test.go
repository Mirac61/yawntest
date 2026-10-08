package scan

import (
	"slices"
	"testing"
)

// testdata holds:
//   - Tested: called from a_test.go
//   - Untested: no test mentions it
//   - OnlyInYawntest: covered by a generated yawntest file
//   - Generated: lives in a "Code generated ... DO NOT EDIT." file
//   - Vendored: lives under vendor/
//   - sub/, z.go: a subdir sorted between files, so Dirs must not list testdata twice
//   - fee.go: no candidates, only a money hint, and must still be returned
func TestUntested(t *testing.T) {
	tests := []struct {
		name          string
		countYawntest bool
		want          []string
	}{
		{name: "yawntest files count as tests", countYawntest: true, want: []string{"Untested"}},
		{name: "yawntest files ignored", countYawntest: false, want: []string{"Untested", "OnlyInYawntest"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files, err := Untested("testdata", test.countYawntest)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}

			var names []string
			hintCount := 0
			for _, file := range files {
				if got, want := file.Package, "pkg"; got != want {
					t.Errorf("%s: Package = %q, want %q", file.Path, got, want)
				}
				for _, match := range file.Matches {
					names = append(names, match.Name)
				}
				hintCount += len(file.Hints)
			}
			if hintCount != 1 {
				t.Errorf("got %d hints, want 1 (defaultFee in a file without candidates)", hintCount)
			}
			if !slices.Equal(names, test.want) {
				t.Errorf("names = %v, want %v", names, test.want)
			}
		})
	}
}

func TestDirs(t *testing.T) {
	tests := []struct {
		name string
		keep func(string) bool
		want []string
	}{
		{name: "yawntest files", keep: IsYawntestFile, want: []string{"testdata"}},
		{name: "source files skip vendor", keep: IsSourceFile, want: []string{"testdata", "testdata/sub"}},
		{name: "nothing", keep: func(string) bool { return false }, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dirs, err := Dirs("testdata", test.keep)
			if err != nil {
				t.Fatalf("dirs: %v", err)
			}
			if !slices.Equal(dirs, test.want) {
				t.Errorf("dirs = %v, want %v", dirs, test.want)
			}
		})
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
