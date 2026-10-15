package report

import (
	"fmt"
	"io"

	"github.com/Mirac61/yawntest/internal/gen"
)

type GeneratedFile struct {
	Source   string
	TestPath string
	Output   gen.Output
	Written  bool // false if TestPath already existed and --force was not set
}

func generated(p *printer, files []GeneratedFile) {
	if len(files) == 0 {
		return
	}
	p.section("Generated")
	p.table(func(table io.Writer) {
		for _, file := range files {
			for _, test := range file.Output.Tests {
				where := location(file.Source, test.Match.Line)
				if file.Written {
					fmt.Fprintf(table, "  %s %s\t%s\t%s\t%s\n", paint(green, "✓"), where, test.Match.Name, test.Match.Pattern, test.Summary)
				} else {
					fmt.Fprintf(table, "  %s %s\t%s\t%s\t%s exists, rerun with --force\n", paint(amber, "!"), where, test.Match.Name, test.Match.Pattern, file.TestPath)
				}
			}
			for _, match := range file.Output.Skipped {
				fmt.Fprintf(table, "  %s %s\t%s\t%s\tskipped, needs inputs yawntest can't build\n", paint(gray, "·"), location(file.Source, match.Line), match.Name, match.Pattern)
			}
		}
	})
}

type generatedCounts struct {
	files, tests, skipped, existing int
}

func countGenerated(files []GeneratedFile) generatedCounts {
	var counts generatedCounts
	for _, file := range files {
		counts.skipped += len(file.Output.Skipped)
		if file.Written {
			counts.files++
			counts.tests += len(file.Output.Tests)
		} else {
			counts.existing++
		}
	}
	return counts
}
