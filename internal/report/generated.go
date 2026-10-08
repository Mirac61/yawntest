package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Mirac61/yawntest/internal/gen"
)

type GeneratedFile struct {
	Source   string
	TestPath string
	Output   gen.Output
	Written  bool // false if TestPath already existed and --force was not set
}

func Generated(w io.Writer, files []GeneratedFile) error {
	if len(files) == 0 {
		_, err := fmt.Fprintln(w, "Nothing to do.")
		return err
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	writtenFiles, writtenTests, skipped := 0, 0, 0
	for _, file := range files {
		fmt.Fprintln(table, file.Source)
		for _, test := range file.Output.Tests {
			if file.Written {
				fmt.Fprintf(table, "  %s %s\t%s\t%s\n", paint(green, "✓"), test.Match.Name, test.Match.Pattern, test.Summary)
			} else {
				fmt.Fprintf(table, "  %s %s\t%s\t%s exists, rerun with --force\n", paint(amber, "!"), test.Match.Name, test.Match.Pattern, file.TestPath)
			}
		}
		for _, match := range file.Output.Skipped {
			fmt.Fprintf(table, "  %s %s\t%s\tskipped, needs inputs yawntest can't build\n", paint(gray, "·"), match.Name, match.Pattern)
		}

		skipped += len(file.Output.Skipped)
		if file.Written {
			writtenFiles++
			writtenTests += len(file.Output.Tests)
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}

	summary := fmt.Sprintf("Wrote %s with %s", plural(writtenFiles, "file"), plural(writtenTests, "test"))
	if skipped > 0 {
		summary += ", skipped " + plural(skipped, "candidate")
	}
	_, err := fmt.Fprintf(w, "\n%s.\n", summary)
	return err
}
