package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/yawntest/internal/scan"
)

// Hints prints nothing when there are none, they are advice on top of the check.
func Hints(w io.Writer, files []scan.File) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	count := 0
	for _, file := range files {
		for _, hint := range file.Hints {
			if count == 0 {
				fmt.Fprintln(table, "\nHints:")
			}
			fmt.Fprintf(table, "  %s:%d\t%s\t%s\n", file.Path, hint.Line, hint.Domain, hint.Message)
			count++
		}
	}
	return table.Flush()
}
