package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/lazytest/internal/scan"
)

func Text(w io.Writer, files []scan.File) error {
	if len(files) == 0 {
		_, err := fmt.Fprintln(w, "Nothing to do.")
		return err
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	matchCount := 0
	for _, file := range files {
		fmt.Fprintln(table, file.Path)
		for _, match := range file.Matches {
			fmt.Fprintf(table, "  %s\t%s\tline %d\t%s\n", match.Name, match.Pattern, match.Line, match.Reason)
			matchCount++
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(w, "\n%s in %s.\n", plural(matchCount, "untested candidate"), plural(len(files), "file"))
	return err
}

func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
