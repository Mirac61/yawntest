package changes

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Mirac61/lazytest/internal/detect"
	"github.com/Mirac61/lazytest/internal/run"
	"github.com/Mirac61/lazytest/internal/scan"
)

func TestParseDiff(t *testing.T) {
	diff := `diff --git a/calc.go b/calc.go
--- a/calc.go
+++ b/calc.go
@@ -3 +3,2 @@ func Add(a, b int) int {
-	return a + b
+	sum := a + b
+	return sum
@@ -10,2 +11,0 @@ func Sub(a, b int) int {
-	// gone
-	// gone too
diff --git a/old.go b/old.go
deleted file mode 100644
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package calc
`
	changed := parseDiff("/repo", diff)

	if len(changed) != 1 {
		t.Fatalf("got %d files, want 1: %v", len(changed), changed)
	}
	var lines []int
	for line := range changed["/repo/calc.go"].Lines {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	if want := []int{3, 4, 11}; !slices.Equal(lines, want) {
		t.Errorf("lines = %v, want %v", lines, want)
	}
}

func TestTouches(t *testing.T) {
	file := File{Lines: map[int]bool{5: true}}
	tests := []struct {
		name       string
		file       File
		start, end int
		want       bool
	}{
		{name: "range contains the change", file: file, start: 3, end: 7, want: true},
		{name: "one-line match on the change", file: file, start: 5, end: 5, want: true},
		{name: "range before the change", file: file, start: 1, end: 4, want: false},
		{name: "untracked file", file: File{All: true}, start: 1, end: 1, want: true},
		{name: "unchanged file", file: File{}, start: 1, end: 100, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.file.Touches(test.start, test.end); got != test.want {
				t.Errorf("Touches(%d, %d) = %v, want %v", test.start, test.end, got, test.want)
			}
		})
	}
}

func TestSinceHEADAndFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("runs git")
	}

	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	writeFile(t, repo, "calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Sub(a, b int) int { return a - b }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "init")

	writeFile(t, repo, "calc.go", "package calc\n\nfunc Add(a, b int) int { return b + a }\n\nfunc Sub(a, b int) int { return a - b }\n")
	writeFile(t, repo, "new.go", "package calc\n\nfunc Mul(a, b int) int { return a * b }\n")

	changed, err := SinceHEAD(repo)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}

	files := []scan.File{
		{Path: filepath.Join(repo, "calc.go"), Matches: []detect.Match{
			{Name: "Add", Line: 3, EndLine: 3},
			{Name: "Sub", Line: 5, EndLine: 5},
		}},
		{Path: filepath.Join(repo, "new.go"), Matches: []detect.Match{{Name: "Mul", Line: 3, EndLine: 3}}},
	}

	byMatch, err := KeepChangedMatches(files, changed)
	if err != nil {
		t.Fatalf("filter matches: %v", err)
	}
	if got, want := matchNames(byMatch), []string{"Add", "Mul"}; !slices.Equal(got, want) {
		t.Errorf("KeepChangedMatches = %v, want %v", got, want)
	}

	byFile, err := KeepChangedFiles(files, changed)
	if err != nil {
		t.Fatalf("filter files: %v", err)
	}
	if got, want := matchNames(byFile), []string{"Add", "Sub", "Mul"}; !slices.Equal(got, want) {
		t.Errorf("KeepChangedFiles = %v, want %v", got, want)
	}

	paths := []run.ErrorPath{
		{File: filepath.Join(repo, "calc.go"), Line: 3},
		{File: filepath.Join(repo, "calc.go"), Line: 5},
	}
	keptPaths, err := KeepChangedErrorPaths(paths, changed)
	if err != nil {
		t.Fatalf("filter error paths: %v", err)
	}
	if len(keptPaths) != 1 || keptPaths[0].Line != 3 {
		t.Errorf("KeepChangedErrorPaths = %+v, want only line 3", keptPaths)
	}
}

func matchNames(files []scan.File) []string {
	var names []string
	for _, file := range files {
		for _, match := range file.Matches {
			names = append(names, match.Name)
		}
	}
	return names
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
