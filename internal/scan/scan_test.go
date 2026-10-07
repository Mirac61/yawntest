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
//   - sub/, z.go: a subdir sorted between files, so Dirs must not list testdata twice
func TestUntested(t *testing.T) {
	tests := []struct {
		name          string
		countLazytest bool
		want          []string
	}{
		{name: "lazytest files count as tests", countLazytest: true, want: []string{"Untested"}},
		{name: "lazytest files ignored", countLazytest: false, want: []string{"Untested", "OnlyInLazytest"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files, err := Untested("testdata", test.countLazytest)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if len(files) != 1 {
				t.Fatalf("got %d files, want 1", len(files))
			}

			file := files[0]
			if got, want := file.Package, "pkg"; got != want {
				t.Errorf("Package = %q, want %q", got, want)
			}

			var names []string
			for _, match := range file.Matches {
				names = append(names, match.Name)
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
		{name: "lazytest files", keep: IsLazytestFile, want: []string{"testdata"}},
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
