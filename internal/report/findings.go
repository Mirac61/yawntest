package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/lazytest/internal/run"
)

func Findings(w io.Writer, findings []run.Finding) error {
	if len(findings) == 0 {
		_, err := fmt.Fprintln(w, "\nAll generated tests pass.")
		return err
	}

	fmt.Fprintln(w, "\nFindings (generated tests that fail on the current code):")
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, finding := range findings {
		test := finding.Test
		if test == "" {
			test = "build failed"
		}
		fmt.Fprintf(table, "  ⚠ %s\t%s\t%s\n", finding.Dir, test, finding.Message)
	}
	if err := table.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(w, "\n%s.\n", plural(len(findings), "finding"))
	return err
}
