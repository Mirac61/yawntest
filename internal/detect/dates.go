package detect

import (
	"fmt"
	"go/ast"
	"go/token"
)

// Methods whose result depends on the time zone. Deadlines, durations and
// comparisons (Add, Sub, Before, Unix) don't, so time.Now() passed on or used
// for timing is left alone.
// ponytail: only direct chains; now := time.Now() followed by now.Year() is missed.
var calendarMethods = map[string]bool{
	"Year": true, "Month": true, "Day": true, "Weekday": true, "YearDay": true, "ISOWeek": true,
	"Date": true, "Clock": true, "Hour": true, "Minute": true, "AddDate": true,
	"Format": true, "AppendFormat": true,
}

func nowWithoutLocation(fset *token.FileSet, file *ast.File, imports importTable) []Hint {
	var hints []Hint
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || !calendarMethods[selector.Sel.Name] {
			return true
		}
		call, ok := selector.X.(*ast.CallExpr)
		if !ok || !imports.isMember(call.Fun, "time", "Now") {
			return true
		}

		hints = append(hints, Hint{
			Line:    fset.Position(call.Pos()).Line,
			Domain:  "dates",
			Message: fmt.Sprintf("time.Now().%s() depends on the server's time zone; call .UTC() or .In(loc) first", selector.Sel.Name),
		})
		return true
	})
	return hints
}
