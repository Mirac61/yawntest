package gen

import (
	"flag"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mirac61/lazytest/internal/detect"
	"github.com/Mirac61/lazytest/internal/scan"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

func TestGolden(t *testing.T) {
	for _, fixture := range fixtures(t) {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			got := generate(t, fixture)

			golden := strings.TrimSuffix(fixture, ".go") + ".golden"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// Puts every fixture with its generated tests into one package, like a real project with
// several lazytest files side by side, and runs go vet and go test there.
func TestGeneratedCodeCompilesAndPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.22\n"))
	for _, fixture := range fixtures(t) {
		source, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		name := filepath.Base(fixture)
		writeFile(t, filepath.Join(dir, name), source)
		writeFile(t, filepath.Join(dir, TestPath(name)), generate(t, fixture))
	}

	runGo(t, dir, "vet", ".")
	runGo(t, dir, "test", ".")
}

func TestTestPath(t *testing.T) {
	if got, want := TestPath("internal/api/tasks.go"), "internal/api/tasks_lazytest_test.go"; got != want {
		t.Errorf("TestPath = %q, want %q", got, want)
	}
}

func fixtures(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("testdata/*.go")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	return paths
}

func generate(t *testing.T, path string) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	out, err := File(file.Name.Name, scan.BuildConstraint(path, file), detect.File(fset, file))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return out.Code
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
