package detect

import (
	"fmt"
	"go/ast"
	"go/token"
)

var secretWords = map[string]bool{"password": true, "token": true, "secret": true, "session": true}

// secretComparisons flags == and != on secrets, which leak timing; nil and "" checks are fine.
func secretComparisons(fset *token.FileSet, file *ast.File) []Hint {
	var hints []Hint
	ast.Inspect(file, func(node ast.Node) bool {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok || binary.Op != token.EQL && binary.Op != token.NEQ {
			return true
		}
		if isEmptyCheck(binary.X) || isEmptyCheck(binary.Y) {
			return true
		}

		name := secretName(binary.X)
		if name == "" {
			name = secretName(binary.Y)
		}
		if name != "" {
			hints = append(hints, Hint{
				Line:    fset.Position(binary.Pos()).Line,
				Domain:  "auth",
				Message: fmt.Sprintf("%s compared with %s; use subtle.ConstantTimeCompare so the timing doesn't leak it", name, binary.Op),
			})
		}
		return true
	})
	return hints
}

// secretName returns the identifier if it names a secret: token, user.Password, sessionID.
func secretName(expr ast.Expr) string {
	var name string
	switch expr := expr.(type) {
	case *ast.Ident:
		name = expr.Name
	case *ast.SelectorExpr:
		name = expr.Sel.Name
	default:
		return ""
	}

	if hasWord(name, secretWords) {
		return name
	}
	return ""
}

func isEmptyCheck(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "nil"
	}
	literal, ok := expr.(*ast.BasicLit)
	return ok && (literal.Value == `""` || literal.Kind == token.INT)
}
