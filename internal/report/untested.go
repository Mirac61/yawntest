package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/scan"
)

// candidate is one row: a declaration with every pattern that matched it.
type candidate struct {
	path, name string
	line       int
	patterns   []string
	reason     string
}

func untested(p *printer, files []scan.File) {
	rows := candidates(files)
	if len(rows) == 0 {
		return
	}
	p.section("Untested")
	p.table(func(table io.Writer) {
		for _, row := range rows {
			fmt.Fprintf(table, "  %s\t%s\t%s\t%s\n", location(row.path, row.line), row.name, strings.Join(row.patterns, ", "), row.reason)
		}
	})
}

// candidates folds the matches of one declaration into one row. ParseStatus as a pure-func and
// as an enum-roundtrip is one thing to test, so it's one line; the reason of the more specific
// pattern says more than "basic params, no I/O".
func candidates(files []scan.File) []candidate {
	var rows []candidate
	for _, file := range files {
		index := map[string]int{}
		for _, match := range file.Matches {
			key := fmt.Sprintf("%s:%d", match.Name, match.Line)
			if i, seen := index[key]; seen {
				rows[i].patterns = append(rows[i].patterns, string(match.Pattern))
				if match.Pattern != detect.PatternPureFunc {
					rows[i].reason = match.Reason
				}
				continue
			}
			index[key] = len(rows)
			rows = append(rows, candidate{
				path:     file.Path,
				name:     match.Name,
				line:     match.Line,
				patterns: []string{string(match.Pattern)},
				reason:   match.Reason,
			})
		}
	}
	return rows
}

func matchCount(files []scan.File) int {
	count := 0
	for _, file := range files {
		count += len(file.Matches)
	}
	return count
}
