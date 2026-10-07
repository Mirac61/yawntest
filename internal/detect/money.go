package detect

import (
	"fmt"
	"go/ast"
	"go/token"
)

var moneyWords = map[string]bool{"amount": true, "price": true, "total": true, "cost": true, "balance": true, "fee": true}

// floatMoney flags struct fields, params, results and vars with a money name and a float type.
func floatMoney(fset *token.FileSet, file *ast.File) []Hint {
	var hints []Hint
	check := func(names []*ast.Ident, typ ast.Expr) {
		floatType := floatTypeName(typ)
		if floatType == "" {
			return
		}
		for _, name := range names {
			if hasWord(name.Name, moneyWords) {
				hints = append(hints, Hint{
					Line:    fset.Position(name.Pos()).Line,
					Domain:  "money",
					Message: fmt.Sprintf("%s is a %s; floats can't hold cents exactly, use integer cents or a decimal type", name.Name, floatType),
				})
			}
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Field:
			check(node.Names, node.Type)
		case *ast.ValueSpec:
			check(node.Names, node.Type)
		}
		return true
	})
	return hints
}

func floatTypeName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name != "float32" && ident.Name != "float64" {
		return ""
	}
	return ident.Name
}
