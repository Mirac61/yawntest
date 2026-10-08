package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// ZeroReceiverNote ends the panic message gen writes for methods. A failure carrying it most
// likely means the receiver lacks dependencies, not that the code has a bug.
const ZeroReceiverNote = "is zero-valued; give it real dependencies if it needs them"

type Finding struct {
	Dir        string
	Test       string // e.g. TestYawntest_CreateTask_BadInput/invalid_json; empty if the package didn't build
	Message    string
	NeedsSetup bool // a method panicked on its zero-valued receiver
}

// ponytail: one go test per dir, so nested modules work; batch by module if this gets slow.
func YawntestTests(dirs []string) ([]Finding, error) {
	var findings []Finding
	for _, dir := range dirs {
		dirFindings, err := yawntestTestsIn(dir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, dirFindings...)
	}
	return findings, nil
}

func yawntestTestsIn(dir string) ([]Finding, error) {
	cmd := exec.Command("go", "test", "-json", "-run", "Yawntest", ".")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil, fmt.Errorf("go test in %s: %w", dir, err)
	}

	findings, parseErr := parseTestEvents(&stdout)
	if parseErr != nil {
		return nil, fmt.Errorf("read go test output in %s: %w", dir, parseErr)
	}
	if err != nil && len(findings) == 0 {
		findings = []Finding{{Message: firstLine(stderr.String())}}
	}

	for i := range findings {
		findings[i].Dir = dir
	}
	return findings, nil
}

type testEvent struct {
	Action string
	Test   string
	Output string
}

func parseTestEvents(r io.Reader) ([]Finding, error) {
	outputByTest := map[string][]string{}
	var failed []string
	var buildOutput strings.Builder
	buildFailed := false

	decoder := json.NewDecoder(r)
	for {
		var event testEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		switch {
		case event.Action == "build-output":
			buildOutput.WriteString(event.Output)
		case event.Action == "build-fail":
			buildFailed = true
		case event.Test == "":
			continue
		case event.Action == "output":
			outputByTest[event.Test] = append(outputByTest[event.Test], event.Output)
		case event.Action == "fail":
			failed = append(failed, event.Test)
		}
	}

	if buildFailed {
		return []Finding{{Message: firstLine(buildOutput.String())}}, nil
	}

	var findings []Finding
	for _, test := range failed {
		if hasFailedSubtest(test, failed) {
			continue
		}
		message := messageFor(test, outputByTest)
		findings = append(findings, Finding{
			Test:       test,
			Message:    message,
			NeedsSetup: strings.Contains(message, ZeroReceiverNote),
		})
	}
	return findings, nil
}

// A parent test fails with its subtests; only the subtest says what went wrong.
func hasFailedSubtest(test string, failed []string) bool {
	return slices.ContainsFunc(failed, func(other string) bool {
		return strings.HasPrefix(other, test+"/")
	})
}

// A panic in a subtest (e.g. a fuzz seed) is printed under its parent test.
func messageFor(test string, outputByTest map[string][]string) string {
	name := test
	for {
		if message := failureMessage(outputByTest[name]); message != "" {
			return message
		}
		slash := strings.LastIndex(name, "/")
		if slash < 0 {
			return "failed"
		}
		name = name[:slash]
	}
}

// failureMessage drops go test's own lines and the "file.go:35: " prefix of t.Errorf.
func failureMessage(output []string) string {
	for _, line := range output {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=== ") || strings.HasPrefix(line, "--- ") {
			continue
		}
		if _, message, found := strings.Cut(line, ".go:"); found {
			if _, text, ok := strings.Cut(message, ": "); ok {
				return text
			}
		}
		return line
	}
	return ""
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return "go test failed"
}
