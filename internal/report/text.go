package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/lazytest/internal/detect"
)

func Text(w io.Writer, matches []detect.Match) error {
	if len(matches) == 0 {
		_, err := fmt.Fprintln(w, "Nothing to do.")
		return err
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fileCount := 0
	for i, match := range matches {
		isNewFile := i == 0 || matches[i-1].File != match.File
		if isNewFile {
			fmt.Fprintln(table, match.File)
			fileCount++
		}
		fmt.Fprintf(table, "  %s\t%s\tline %d\t%s\n", match.Name, match.Pattern, match.Line, match.Reason)
	}
	if err := table.Flush(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(w, "\n%s in %s.\n", plural(len(matches), "untested candidate"), plural(fileCount, "file"))
	return err
}

func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
