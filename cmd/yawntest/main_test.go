package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

// binary is the yawntest built once for every end-to-end test.
var binary string

// TestMain builds the real binary: these tests run it like a user would, exit codes included.
// The version is fixed, or every commit would change the SARIF golden.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	dir, err := os.MkdirTemp("", "yawntest-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "yawntest")
	if output, err := exec.Command("go", "build", "-ldflags", "-X main.stampedVersion=e2e", "-o", binary, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build yawntest: %v\n%s", err, output)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestEndToEnd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  []string // $WORK is replaced by the temp dir holding the example
	}{
		{name: "check", args: []string{"check", "./..."}},
		{name: "check_json", args: []string{"check", "--json", "./..."}},
		{name: "check_github", args: []string{"check", "--format", "github", "./..."}, env: []string{"GITHUB_WORKSPACE=$WORK"}},
		{name: "check_static", args: []string{"check", "--static", "./..."}},
		{name: "check_sarif", args: []string{"check", "--format", "sarif", "./..."}, env: []string{"GITHUB_WORKSPACE=$WORK"}},
		{name: "gen", args: []string{"gen", "./..."}},
		{name: "gen_all", args: []string{"gen", "--all", "./..."}}, // ParseQuantity has a test, --all writes one anyway
		{name: "run", args: []string{"run", "./..."}},
		{name: "usage_error", args: []string{"check", "a", "b"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			work, example := copyExample(t)
			var env []string
			for _, variable := range test.env {
				env = append(env, strings.ReplaceAll(variable, "$WORK", work))
			}
			checkGolden(t, test.name, work, yawntest(t, example, env, test.args...))
		})
	}
}

// --sarif writes a SARIF file next to the normal output.
func TestSARIFFile(t *testing.T) {
	_, example := copyExample(t)
	sarif := filepath.Join(t.TempDir(), "out.sarif")
	if got := yawntest(t, example, nil, "check", "--static", "--sarif", sarif, "./..."); !strings.Contains(got, "Untested") {
		t.Errorf("text output missing:\n%s", got)
	}
	data, err := os.ReadFile(sarif)
	if err != nil || !strings.Contains(string(data), `"ruleId": "untested/enum-roundtrip"`) {
		t.Errorf("SARIF file: %v\n%s", err, data)
	}
}

// .yawntest.json leaves out a file and turns off a pattern and a hint; a typo in it is exit 2.
func TestConfig(t *testing.T) {
	work, example := copyExample(t)
	config := filepath.Join(example, ".yawntest.json")
	writeFile(t, config, `{"exclude": ["shop/user.go"], "disable": ["pattern:enum-roundtrip", "hint:money"]}`)
	checkGolden(t, "check_config", work, yawntest(t, example, nil, "check", "--static", "./..."))

	writeFile(t, config, `{"disable": ["hint:mony"]}`)
	checkGolden(t, "check_config_error", work, yawntest(t, example, nil, "check", "./..."))
}

// A second gen without --force writes nothing, and --force writes the same bytes again.
func TestGenIsIdempotent(t *testing.T) {
	_, example := copyExample(t)
	yawntest(t, example, nil, "gen", "./...")
	before := readYawntestFiles(t, example)

	if got := yawntest(t, example, nil, "gen", "./..."); !strings.Contains(got, "Nothing to do") {
		t.Errorf("second gen:\n%s\nwant Nothing to do", got)
	}
	generated, err := filepath.Glob(filepath.Join(example, "shop", "*_yawntest_test.go"))
	if err != nil || len(generated) == 0 {
		t.Fatalf("no generated files: %v", err)
	}
	appendFile(t, generated[0], "\n// edited by hand\n")
	if got := yawntest(t, example, nil, "gen", "--force", "./..."); !strings.HasPrefix(got, "exit 0\n") {
		t.Errorf("gen --force:\n%s", got)
	}
	if after := readYawntestFiles(t, example); after != before {
		t.Error("gen --force didn't write the same files again")
	}
}

// --changed in a git repo: only the func added after the commit shows up.
func TestCheckChanged(t *testing.T) {
	work, example := copyExample(t)
	git(t, example, "init", "-q")
	git(t, example, "add", ".")
	git(t, example, "-c", "user.email=e2e@example.com", "-c", "user.name=e2e", "commit", "-q", "-m", "example")

	appendFile(t, filepath.Join(example, "shop", "order.go"), "\nfunc Double(n int) int { return 2 * n }\n")
	checkGolden(t, "check_changed", work, yawntest(t, example, nil, "check", "--changed", "--static", "./..."))
}

// copyExample copies _example into a fresh temp dir, so tests can write into it.
func copyExample(t *testing.T) (work, example string) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the yawntest binary and the go tool")
	}
	work = t.TempDir()
	example = filepath.Join(work, "example")
	err := filepath.WalkDir("../../_example", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("../../_example", path)
		if err != nil {
			return err
		}
		target := filepath.Join(example, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(path, "_yawntest_test.go") {
			return nil // left over from a manual run
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy example: %v", err)
	}
	return work, example
}

// yawntest runs the binary in dir and returns its exit code, stdout and stderr as one text.
func yawntest(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "NO_COLOR=1", "GITHUB_WORKSPACE="), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run yawntest: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return fmt.Sprintf("exit %d\n--- stdout\n%s--- stderr\n%s", code, stdout.String(), stderr.String())
}

func checkGolden(t *testing.T, name, work, got string) {
	t.Helper()
	// On macOS the binary may see the temp dir as /private/var/… where the test has /var/….
	if resolved, err := filepath.EvalSymlinks(work); err == nil {
		got = strings.ReplaceAll(got, resolved, "$WORK")
	}
	got = strings.ReplaceAll(got, work, "$WORK")
	golden := filepath.Join("testdata", name+".golden")
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
}

func readYawntestFiles(t *testing.T, dir string) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "shop", "*_yawntest_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no yawntest files in %s: %v", dir, err)
	}
	var all strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(data)
	}
	return all.String()
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
