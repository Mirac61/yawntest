package report

// Color turns on ANSI colors; main sets it for terminals. Every code has the same length,
// so tabwriter columns stay aligned when each cell of a column gets one.
var Color bool

const (
	red   = "\x1b[31m"
	green = "\x1b[32m"
	amber = "\x1b[33m"
	gray  = "\x1b[90m"
	reset = "\x1b[0m"
)

func paint(color, text string) string {
	if !Color {
		return text
	}
	return color + text + reset
}
