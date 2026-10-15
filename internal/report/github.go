package report

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GitHub prints the result as workflow commands, which GitHub Actions turns into annotations
// on the lines of a pull request. The runner hides the commands themselves from the log.
func GitHub(w io.Writer, result Result) error {
	a := annotator{w: w, workspace: os.Getenv("GITHUB_WORKSPACE")}
	for _, file := range result.Untested {
		for _, match := range file.Matches {
			a.write("notice", file.Path, match.Line, "yawntest: untested "+string(match.Pattern),
				fmt.Sprintf("%s has no test yet (%s). `yawntest gen` writes one.", match.Name, match.Reason))
		}
		for _, hint := range file.Hints {
			a.write("warning", file.Path, hint.Line, "yawntest: "+hint.Domain, hint.Message)
		}
	}
	for _, path := range result.ErrorPaths {
		a.write("warning", path.File, path.Line, "yawntest: untested error path", "No test reaches this return: "+path.Code)
	}
	real, _ := splitFindings(result.Findings)
	for _, failure := range group(real, result.Generated) {
		a.write("error", failure.file, failure.line, "yawntest finding: "+failure.name, failure.message)
	}
	for _, warning := range result.Warnings {
		a.write("warning", "", 0, "yawntest", warning)
	}
	return a.err
}

type annotator struct {
	w         io.Writer
	workspace string // paths in annotations are relative to it
	err       error
}

func (a *annotator) write(level, file string, line int, title, message string) {
	if a.err != nil {
		return
	}
	var properties []string
	if file != "" {
		properties = append(properties, "file="+escapeProperty(a.relative(file)))
		if line > 0 {
			properties = append(properties, fmt.Sprintf("line=%d", line))
		}
	}
	properties = append(properties, "title="+escapeProperty(title))
	_, a.err = fmt.Fprintf(a.w, "::%s %s::%s\n", level, strings.Join(properties, ","), escapeData(message))
}

func (a *annotator) relative(path string) string {
	if a.workspace == "" {
		return filepath.ToSlash(path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	relative, err := filepath.Rel(a.workspace, absolute)
	if err != nil || strings.HasPrefix(relative, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func escapeData(s string) string     { return dataEscaper.Replace(s) }
func escapeProperty(s string) string { return propertyEscaper.Replace(s) }
