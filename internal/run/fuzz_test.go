package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The seeds of FuzzYawntest_Check pass; only fuzzing finds the "z.." input that panics.
func TestFuzz(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go fuzzer")
	}
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module fz\n\ngo 1.22\n")
	writeFile(t, dir, "check.go", `package fz

func Check(s string) int {
	if len(s) == 3 && s[0] == 'z' {
		panic("boom")
	}
	return len(s)
}

func Fine(n int) int { return n }
`)
	writeFile(t, dir, "check_yawntest_test.go", `package fz

import "testing"

func FuzzYawntest_Check(f *testing.F) {
	f.Add("")
	f.Add("abc")
	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v", r)
			}
		}()
		Check(s)
	})
}

func FuzzYawntest_Fine(f *testing.F) {
	f.Add(1)
	f.Fuzz(func(t *testing.T, n int) { Fine(n) })
}

func TestYawntest_NotFuzz(t *testing.T) {}
`)

	if seeds, err := YawntestTests([]string{dir}, nil); err != nil || len(seeds) != 0 {
		t.Fatalf("seed run = %+v, %v; want the seeds to pass", seeds, err)
	}
	targets, err := FuzzTargets([]string{dir})
	if err != nil || len(targets) != 2 || targets[0].Name != "FuzzYawntest_Check" || targets[1].Name != "FuzzYawntest_Fine" {
		t.Fatalf("targets = %+v, %v", targets, err)
	}

	finding, failed, err := Fuzz(targets[0], 20*time.Second)
	if err != nil || !failed {
		t.Fatalf("Fuzz = %+v, %v, %v; want a finding", finding, failed, err)
	}
	if !strings.HasPrefix(finding.Test, "FuzzYawntest_Check/") || !strings.HasPrefix(finding.Message, "panic: boom (input saved in ") {
		t.Errorf("finding = %+v", finding)
	}
	// go keeps the failing input as a regression test, and the finding's Test runs exactly it.
	input := filepath.Join(dir, "testdata", "fuzz", "FuzzYawntest_Check", strings.TrimPrefix(finding.Test, "FuzzYawntest_Check/"))
	if _, err := os.Stat(input); err != nil {
		t.Errorf("failing input not saved: %v", err)
	}

	if finding, failed, err := Fuzz(targets[1], time.Second); err != nil || failed {
		t.Errorf("Fuzz(Fine) = %+v, %v, %v; want no finding", finding, failed, err)
	}
}

func TestFuzzEstimate(t *testing.T) {
	if got, want := FuzzEstimate(6, 10*time.Second), "fuzzing 6 targets for 10s each (~1m0s)"; got != want {
		t.Errorf("FuzzEstimate = %q, want %q", got, want)
	}
}
