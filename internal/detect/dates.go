package detect

import (
	"go/ast"
	"go/token"
)

// Calls on time.Now() whose result doesn't depend on the time zone.
var zoneSafeMethods = map[string]bool{
	"UTC": true, "In": true,
	"Unix": true, "UnixMilli": true, "UnixMicro": true, "UnixNano": true,
	"Sub": true, "Before": true, "After": true, "Equal": true, "Compare": true,
}

func nowWithoutLocation(fset *token.FileSet, file *ast.File, imports importTable) []Hint {
	zoneSafe := map[*ast.CallExpr]bool{}
	var nowCalls []*ast.CallExpr

	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.SelectorExpr:
			call, ok := node.X.(*ast.CallExpr)
			if ok && zoneSafeMethods[node.Sel.Name] {
				zoneSafe[call] = true
			}
		case *ast.CallExpr:
			if imports.isMember(node.Fun, "time", "Now") {
				nowCalls = append(nowCalls, node)
			}
		}
		return true
	})

	var hints []Hint
	for _, call := range nowCalls {
		if zoneSafe[call] {
			continue
		}
		hints = append(hints, Hint{
			Line:    fset.Position(call.Pos()).Line,
			Domain:  "dates",
			Message: "time.Now() uses the server's local time zone; call .UTC() or .In(loc) before working with the date",
		})
	}
	return hints
}
