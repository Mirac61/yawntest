package report

import "strings"

// summary is the last line: what was found, in one line, and the command to run next.
func summary(p *printer, result Result, view View) {
	var parts []string
	next := ""

	switch view.Command {
	case "check":
		candidates := matchCount(result.Untested)
		if candidates > 0 {
			parts = append(parts, paint(amber, plural(candidates, "untested candidate")))
			next = "yawntest gen writes the tests"
		}
		if hints := hintCount(result.Untested); hints > 0 {
			parts = append(parts, plural(hints, "hint"))
		}
		if !view.Static && len(result.ErrorPaths) > 0 {
			parts = append(parts, paint(red, plural(len(result.ErrorPaths), "untested error path")))
		}
		if candidates == 0 && (view.Static || len(result.ErrorPaths) == 0) {
			parts = append([]string{paint(green, "Nothing untested")}, parts...)
		}
	default:
		counts := countGenerated(result.Generated)
		switch {
		case counts.files > 0:
			wrote := "Wrote " + plural(counts.files, "file") + " with " + plural(counts.tests, "test")
			if counts.skipped > 0 {
				wrote += ", skipped " + plural(counts.skipped, "candidate")
			}
			parts = append(parts, wrote)
			next = "yawntest run runs them"
		case counts.existing > 0:
			parts = append(parts, "Nothing written, the yawntest files exist; rerun with --force")
		default:
			parts = append(parts, "Nothing to do")
		}
		if view.Command == "run" {
			// The tests already ran, and each finding carries its own rerun command.
			parts, next = append(parts, findingsSummary(result)...), ""
		}
	}

	line := strings.Join(parts, " · ")
	if next != "" {
		line += paint(gray, "  → "+next)
	}
	p.section(line)
}

// findingsSummary counts what run found.
func findingsSummary(result Result) []string {
	real, needsSetup := splitFindings(result.Findings)
	bugs, setup := group(real, result.Generated), group(needsSetup, result.Generated)

	var parts []string
	if len(bugs) == 0 {
		parts = append(parts, paint(green, "all generated tests pass"))
	} else {
		parts = append(parts, paint(red, plural(len(bugs), "finding")))
	}
	if len(setup) > 0 {
		parts = append(parts, plural(len(setup), "test")+" without setup")
	}
	return parts
}
