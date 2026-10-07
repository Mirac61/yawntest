package detect

import (
	"go/ast"
	"slices"
	"strings"
)

var ioPackages = map[string]bool{
	"bufio": true, "io": true, "io/fs": true, "io/ioutil": true,
	"os": true, "os/exec": true, "syscall": true,
	"net": true, "net/http": true, "database/sql": true,
	"log": true, "log/slog": true,
	"crypto/rand": true, "math/rand": true, "math/rand/v2": true,
}

var impureTimeFuncs = map[string]bool{
	"Now": true, "Since": true, "Until": true, "Sleep": true,
	"After": true, "AfterFunc": true, "Tick": true, "NewTimer": true, "NewTicker": true,
}

var fmtIOPrefixes = []string{"Print", "Fprint", "Scan", "Fscan"}

func detectPureFunc(fn *ast.FuncDecl, imports importTable) (string, bool) {
	isPlainFunc := fn.Recv == nil && fn.Name.IsExported() && fn.Type.TypeParams == nil
	if !isPlainFunc {
		return "", false
	}
	// Without a result there is nothing to compare for determinism.
	if fn.Type.Results == nil {
		return "", false
	}

	params := fieldTypes(fn.Type.Params)
	if len(params) == 0 {
		return "", false
	}
	for _, param := range params {
		if !isBasicParam(param, imports) {
			return "", false
		}
	}

	if hasSideEffects(fn.Body, imports) {
		return "", false
	}
	return "basic params, no I/O", true
}

func isBasicParam(expr ast.Expr, imports importTable) bool {
	switch typ := expr.(type) {
	case *ast.ArrayType:
		isSlice := typ.Len == nil
		return isSlice && isBasicScalar(typ.Elt, imports)
	case *ast.Ellipsis:
		return isBasicScalar(typ.Elt, imports)
	default:
		return isBasicScalar(expr, imports)
	}
}

func isBasicScalar(expr ast.Expr, imports importTable) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return basicTypes[ident.Name]
	}
	return imports.isMember(expr, "time", "Time")
}

// ponytail: misses I/O hidden in called funcs; needs go/types.
func hasSideEffects(body *ast.BlockStmt, imports importTable) bool {
	return containsNode(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.GoStmt:
			return true
		case *ast.SelectorExpr:
			pkgPath, member, ok := imports.resolve(node)
			return ok && isImpureMember(pkgPath, member)
		default:
			return false
		}
	})
}

func isImpureMember(pkgPath, member string) bool {
	switch pkgPath {
	case "time":
		return impureTimeFuncs[member]
	case "fmt":
		return slices.ContainsFunc(fmtIOPrefixes, func(prefix string) bool {
			return strings.HasPrefix(member, prefix)
		})
	default:
		return ioPackages[pkgPath]
	}
}
