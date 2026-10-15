package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/Mirac61/yawntest/internal/run"
)

// noTests are dirs without any test; their error returns are summed up, not listed.
func ErrorPaths(w io.Writer, paths []run.ErrorPath, noTests []string) error {
	if len(noTests) > 0 {
		fmt.Fprintf(w, "\nNo tests at all, so no error paths listed: %s\n", strings.Join(noTests, ", "))
	}
	if len(paths) == 0 {
		message := "Every error return worth a test is reached by one."
		if len(noTests) > 0 {
			message = "Every error return worth a test in the tested packages is reached by one."
		}
		_, err := fmt.Fprintln(w, "\n"+message)
		return err
	}

	fmt.Fprintln(w, "\nError returns no test reaches:")
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, path := range paths {
		fmt.Fprintf(table, "  %s:%d\t%s\n", path.File, path.Line, path.Code)
	}
	if err := table.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(w, "\n%s.\n", plural(len(paths), "untested error path"))
	return err
}
