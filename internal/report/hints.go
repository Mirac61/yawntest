package report

import (
	"fmt"
	"io"

	"github.com/Mirac61/yawntest/internal/scan"
)

// Hints are advice on top of the check and never change the exit code.
func hints(p *printer, files []scan.File) {
	if hintCount(files) == 0 {
		return
	}
	p.section("Hints")
	p.table(func(table io.Writer) {
		for _, file := range files {
			for _, hint := range file.Hints {
				fmt.Fprintf(table, "  %s\t%s\t%s\n", location(file.Path, hint.Line), paint(amber, hint.Domain), hint.Message)
			}
		}
	})
}

func hintCount(files []scan.File) int {
	count := 0
	for _, file := range files {
		count += len(file.Hints)
	}
	return count
}
