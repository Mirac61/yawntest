package report

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// issue is one thing to point at in the code. GitHub annotations and SARIF are both made from
// the same list, so they can't drift apart.
type issue struct {
	rule    string // untested/pure-func, hint/money, error-path, finding, warning
	level   string // notice, warning or error, as GitHub calls them
	file    string // empty for warnings about yawntest's own run
	line    int
	title   string
	message string
}

// rank orders issues by how much they matter, for when GitHub shows only some of them.
var rank = map[string]int{"finding": 0, "error-path": 1, "hint": 2, "untested": 3, "warning": 4}

func issues(result Result) []issue {
	var all []issue
	for _, file := range result.Untested {
		for _, match := range file.Matches {
			all = append(all, issue{
				rule: "untested/" + string(match.Pattern), level: "notice", file: file.Path, line: match.Line,
				title:   "yawntest: untested " + string(match.Pattern),
				message: fmt.Sprintf("%s has no test yet (%s). `yawntest gen` writes one.", match.Name, match.Reason),
			})
		}
		for _, hint := range file.Hints {
			all = append(all, issue{
				rule: "hint/" + hint.Domain, level: "warning", file: file.Path, line: hint.Line,
				title: "yawntest: " + hint.Domain, message: hint.Message,
			})
		}
	}
	for _, path := range result.ErrorPaths {
		all = append(all, issue{
			rule: "error-path", level: "warning", file: path.File, line: path.Line,
			title: "yawntest: untested error path", message: "No test reaches this return in " + path.Func + ": " + path.Code,
		})
	}
	real, _ := splitFindings(result.Findings)
	for _, failure := range group(real, result.Generated) {
		all = append(all, issue{
			rule: "finding", level: "error", file: failure.file, line: failure.line,
			title: "yawntest finding: " + failure.name, message: failure.message + "\nRerun: " + failure.rerun,
		})
	}
	for _, warning := range result.Warnings {
		all = append(all, issue{rule: "warning", level: "warning", title: "yawntest", message: warning})
	}
	return all
}

// GitHub shows 10 annotations per level and step and drops the rest without a word.
const annotationsPerLevel = 10

// GitHub prints the result as workflow commands, which GitHub Actions turns into annotations
// on the lines of a pull request. The runner hides the commands themselves from the log.
// The most important issues go first, and a last notice says how many didn't fit.
func GitHub(w io.Writer, result Result) error {
	all := issues(result)
	slices.SortStableFunc(all, func(a, b issue) int { return rank[ruleKind(a.rule)] - rank[ruleKind(b.rule)] })

	// Decide the limits first: if anything is dropped, the notice saying so needs a slot of
	// its own among the notices, or GitHub drops that one too.
	total := map[string]int{}
	for _, issue := range all {
		total[issue.level]++
	}
	limit := map[string]int{"notice": annotationsPerLevel, "warning": annotationsPerLevel, "error": annotationsPerLevel}
	if total["notice"] > annotationsPerLevel || total["warning"] > annotationsPerLevel || total["error"] > annotationsPerLevel {
		limit["notice"]--
	}

	a := annotator{w: w, workspace: os.Getenv("GITHUB_WORKSPACE")}
	shown := map[string]int{}
	dropped := 0
	for _, issue := range all {
		if shown[issue.level] >= limit[issue.level] {
			dropped++
			continue
		}
		shown[issue.level]++
		a.write(issue)
	}
	if dropped > 0 {
		a.write(issue{
			level: "notice", title: "yawntest",
			message: fmt.Sprintf("%d more %s not shown here: GitHub shows %d per kind and step. "+
				"The job summary has the full report; --format sarif with code scanning shows every one.",
				dropped, strings.TrimPrefix(plural(dropped, "issue"), fmt.Sprint(dropped)+" "), annotationsPerLevel),
		})
	}
	return a.err
}

func ruleKind(rule string) string {
	kind, _, _ := strings.Cut(rule, "/")
	return kind
}

type annotator struct {
	w         io.Writer
	workspace string // paths in annotations are relative to it
	err       error
}

func (a *annotator) write(issue issue) {
	if a.err != nil {
		return
	}
	var properties []string
	if issue.file != "" {
		properties = append(properties, "file="+escapeProperty(repoPath(a.workspace, issue.file)))
		if issue.line > 0 {
			properties = append(properties, fmt.Sprintf("line=%d", issue.line))
		}
	}
	properties = append(properties, "title="+escapeProperty(issue.title))
	_, a.err = fmt.Fprintf(a.w, "::%s %s::%s\n", issue.level, strings.Join(properties, ","), escapeData(issue.message))
}

// repoPath is path relative to the workspace, the repo root in GitHub Actions, with slashes.
// Without a workspace, or outside it, the path stays as it is.
func repoPath(workspace, path string) string {
	if workspace == "" {
		return filepath.ToSlash(path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	// Both sides resolved: on macOS the workspace may be /var/… while the cwd is /private/var/….
	relative, err := filepath.Rel(resolveSymlinks(workspace), resolveSymlinks(absolute))
	if err != nil || strings.HasPrefix(relative, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// resolveSymlinks resolves the longest part of path that exists, so a file that isn't there
// (yet) still ends up under the same resolved dir as its siblings.
func resolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(resolveSymlinks(parent), filepath.Base(path))
}

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func escapeData(s string) string     { return dataEscaper.Replace(s) }
func escapeProperty(s string) string { return propertyEscaper.Replace(s) }
