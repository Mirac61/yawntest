package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/Mirac61/yawntest/internal/run"
)

// noTests are dirs without any test; their error returns are summed up, not listed.
func errorPaths(p *printer, paths []run.ErrorPath, noTests []string) {
	if len(paths) > 0 {
		p.section("Error paths no test reaches")
		p.table(func(table io.Writer) {
			for _, path := range paths {
				fmt.Fprintf(table, "  %s\t%s\t%s\n", location(path.File, path.Line), path.Func, path.Code)
			}
		})
	}
	if len(noTests) > 0 {
		p.section("No tests at all, so no error paths listed")
		p.printf("  %s\n", strings.Join(noTests, ", "))
	}
}
