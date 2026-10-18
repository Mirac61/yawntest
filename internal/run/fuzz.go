package run

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FuzzTarget is one generated fuzz test in one dir.
type FuzzTarget struct {
	Dir  string
	Name string // FuzzYawntest_Split
}

// FuzzTargets lists the generated fuzz tests of dirs, in order.
func FuzzTargets(dirs []string) ([]FuzzTarget, error) {
	var targets []FuzzTarget
	for _, dir := range dirs {
		cmd := exec.Command("go", "test", "-list", "^FuzzYawntest_", ".")
		cmd.Dir = dir
		output, err := cmd.Output()
		if err != nil {
			// A package that doesn't build already has its finding from the seed run.
			continue
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "FuzzYawntest_") {
				targets = append(targets, FuzzTarget{Dir: dir, Name: strings.TrimSpace(line)})
			}
		}
	}
	return targets, nil
}

// Fuzz runs one target for duration with Go's fuzzing engine. go test fuzzes one target per
// call, and each call uses every core, so targets run one after another. A failing input is
// written to testdata/fuzz/<target>/<id> by go itself and stays there as a regression test;
// the finding's Test names it, so the rerun command runs just that input.
func Fuzz(target FuzzTarget, duration time.Duration) (Finding, bool, error) {
	cmd := exec.Command("go", "test", "-run", "^$", "-fuzz", "^"+target.Name+"$", "-fuzztime", duration.String(), ".")
	cmd.Dir = target.Dir
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err == nil {
		return Finding{}, false, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return Finding{}, false, fmt.Errorf("fuzz %s in %s: %w", target.Name, target.Dir, err)
	}

	lines := strings.Split(output.String(), "\n")
	finding := Finding{Dir: target.Dir, Test: target.Name, Message: fuzzMessage(lines)}
	for _, line := range lines {
		if written, ok := strings.CutPrefix(strings.TrimSpace(line), "Failing input written to "); ok {
			finding.Test = target.Name + "/" + filepath.Base(written)
			finding.Message += " (input saved in " + filepath.ToSlash(filepath.Join(target.Dir, written)) + ")"
		}
	}
	finding.NeedsSetup = strings.Contains(finding.Message, ZeroReceiverNote)
	return finding, true, nil
}

// fuzzMessage is the first message after go's "--- FAIL" line, like for the seed run.
func fuzzMessage(lines []string) string {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--- FAIL") {
			if message := failureMessage(lines[i+1:]); message != "" {
				return message
			}
		}
	}
	return goOutputLine(strings.Join(lines, "\n"), errors.New("fuzzing failed"))
}

// FuzzEstimate says how long fuzzing will take, so a long run isn't a surprise.
func FuzzEstimate(targets int, duration time.Duration) string {
	total := (time.Duration(targets) * duration).Round(time.Second)
	return fmt.Sprintf("fuzzing %d targets for %s each (~%s)", targets, duration, total)
}
