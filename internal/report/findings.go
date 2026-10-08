package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/yawntest/internal/run"
)

func Findings(w io.Writer, findings []run.Finding) error {
	real, needsSetup := splitFindings(findings)
	if len(real) == 0 && len(needsSetup) == 0 {
		_, err := fmt.Fprintln(w, "\nAll generated tests pass.")
		return err
	}

	if len(real) > 0 {
		fmt.Fprintln(w, "\nFindings (generated tests that fail on the current code):")
		if err := writeFindings(w, "⚠", real); err != nil {
			return err
		}
	}
	if len(needsSetup) > 0 {
		fmt.Fprintln(w, "\nNeeds setup (methods panicked on a zero-valued receiver, give them real dependencies):")
		if err := writeFindings(w, "·", needsSetup); err != nil {
			return err
		}
	}

	summary := plural(len(real), "finding")
	if len(needsSetup) > 0 {
		summary += fmt.Sprintf(", %s without setup", plural(len(needsSetup), "test"))
	}
	_, err := fmt.Fprintf(w, "\n%s.\n", summary)
	return err
}

func splitFindings(findings []run.Finding) (real, needsSetup []run.Finding) {
	for _, finding := range findings {
		if finding.NeedsSetup {
			needsSetup = append(needsSetup, finding)
		} else {
			real = append(real, finding)
		}
	}
	return real, needsSetup
}

func writeFindings(w io.Writer, marker string, findings []run.Finding) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, finding := range findings {
		test := finding.Test
		if test == "" {
			test = "build failed"
		}
		fmt.Fprintf(table, "  %s %s\t%s\t%s\n", marker, finding.Dir, test, finding.Message)
	}
	return table.Flush()
}
