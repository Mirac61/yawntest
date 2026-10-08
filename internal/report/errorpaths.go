package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/yawntest/internal/run"
)

func ErrorPaths(w io.Writer, paths []run.ErrorPath) error {
	if len(paths) == 0 {
		_, err := fmt.Fprintln(w, "\nEvery error return is reached by a test.")
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
