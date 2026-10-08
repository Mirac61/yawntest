package changes

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

type File struct {
	All   bool // untracked file, every line counts as changed
	Lines map[int]bool
}

func (f File) Touches(start, end int) bool {
	if f.All {
		return true
	}
	for line := start; line <= end; line++ {
		if f.Lines[line] {
			return true
		}
	}
	return false
}

// SinceHEAD returns uncommitted and untracked changes of the repo around dir, keyed by resolved absolute path.
func SinceHEAD(dir string) (map[string]File, error) {
	output, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(output))
	if err != nil {
		return nil, err
	}

	diff, err := git(top, "diff", "--unified=0", "--no-color", "--no-ext-diff", "HEAD")
	if err != nil {
		return nil, err
	}
	changed := parseDiff(top, diff)

	untracked, err := git(top, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(untracked, "\n") {
		if path != "" {
			changed[filepath.Join(top, path)] = File{All: true}
		}
	}
	return changed, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func parseDiff(top, diff string) map[string]File {
	changed := map[string]File{}
	current := ""
	for _, line := range strings.Split(diff, "\n") {
		if newPath, ok := strings.CutPrefix(line, "+++ "); ok {
			current = ""
			if path, ok := strings.CutPrefix(newPath, "b/"); ok {
				current = filepath.Join(top, path)
				changed[current] = File{Lines: map[int]bool{}}
			}
			continue
		}

		groups := hunkHeader.FindStringSubmatch(line)
		if groups == nil || current == "" {
			continue
		}
		start, _ := strconv.Atoi(groups[1])
		count := 1
		if groups[2] != "" {
			count, _ = strconv.Atoi(groups[2])
		}
		// Count 0 means lines were only deleted; the line after the deletion changed its function.
		if count == 0 {
			changed[current].Lines[max(start, 1)] = true
		}
		for line := start; line < start+count; line++ {
			changed[current].Lines[line] = true
		}
	}
	return changed
}

// KeepChangedMatches keeps only matches and hints whose lines changed. Used by check.
func KeepChangedMatches(files []scan.File, changed map[string]File) ([]scan.File, error) {
	var kept []scan.File
	for _, file := range files {
		fileChanges, err := lookup(changed, file.Path)
		if err != nil {
			return nil, err
		}

		var matches []detect.Match
		for _, match := range file.Matches {
			if fileChanges.Touches(match.Line, match.EndLine) {
				matches = append(matches, match)
			}
		}
		var hints []detect.Hint
		for _, hint := range file.Hints {
			if fileChanges.Touches(hint.Line, hint.Line) {
				hints = append(hints, hint)
			}
		}

		file.Matches, file.Hints = matches, hints
		if len(matches) > 0 || len(hints) > 0 {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

// KeepChangedFiles keeps whole files with at least one changed match. Generation
// rewrites a file's yawntest file as a unit, so dropping matches would drop their tests.
func KeepChangedFiles(files []scan.File, changed map[string]File) ([]scan.File, error) {
	changedMatches, err := KeepChangedMatches(files, changed)
	if err != nil {
		return nil, err
	}

	touched := map[string]bool{}
	for _, file := range changedMatches {
		if len(file.Matches) > 0 {
			touched[file.Path] = true
		}
	}

	var kept []scan.File
	for _, file := range files {
		if touched[file.Path] {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

func KeepChangedErrorPaths(paths []run.ErrorPath, changed map[string]File) ([]run.ErrorPath, error) {
	var kept []run.ErrorPath
	for _, path := range paths {
		fileChanges, err := lookup(changed, path.File)
		if err != nil {
			return nil, err
		}
		if fileChanges.Touches(path.Line, path.Line) {
			kept = append(kept, path)
		}
	}
	return kept, nil
}

// Keys are resolved, because git reports /private/var/... where the caller may see /var/... (macOS).
func lookup(changed map[string]File, path string) (File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return File{}, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return File{}, err
	}
	return changed[resolved], nil
}
