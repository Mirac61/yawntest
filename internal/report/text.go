package report

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// View says which parts of a result the text report shows.
type View struct {
	Command string // check, gen or run
	Static  bool   // check --static: no error paths were looked for
}

// Text prints the result for a person: one section per kind of finding, every location as
// path:line so terminals and editors can open it, and one summary line at the end.
func Text(w io.Writer, result Result, view View) error {
	p := &printer{w: w}
	if view.Command == "check" {
		untested(p, result.Untested)
		hints(p, result.Untested)
		if !view.Static {
			errorPaths(p, result.ErrorPaths, result.NoTests)
		}
	} else {
		generated(p, result.Generated)
		if view.Command == "run" {
			findings(p, result.Findings, result.Generated)
		}
	}
	summary(p, result, view)
	return p.err
}

// printer writes sections with a blank line between them and keeps the first write error.
type printer struct {
	w       io.Writer
	started bool
	err     error
}

func (p *printer) section(title string) {
	if p.started {
		p.printf("\n")
	}
	p.started = true
	p.printf("%s\n", title)
}

func (p *printer) printf(format string, args ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format, args...)
	}
}

// table aligns the rows of one section; every cell of a column must get the same color, or
// tabwriter counts the escape codes into some widths and not others.
func (p *printer) table(write func(table io.Writer)) {
	if p.err != nil {
		return
	}
	table := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)
	write(table)
	p.err = table.Flush()
}

func location(path string, line int) string {
	return paint(gray, fmt.Sprintf("%s:%d", path, line))
}

func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
