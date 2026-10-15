package detect

import (
	"go/ast"
	"go/token"
	"strings"
)

// ErrorReturn is a return statement that hands back an error a plain unit test can trigger.
type ErrorReturn struct {
	Line, Column int
	Code         string // the return statement, first line only
}

// Packages whose error returns depend on the input alone: a test triggers them with a bad
// string, no fake file system or server needed.
var inputErrorPackages = map[string]bool{
	"strconv": true, "encoding/json": true, "encoding/xml": true, "encoding/hex": true,
	"encoding/base64": true, "encoding/base32": true, "encoding/csv": true, "encoding/asn1": true,
	"net/url": true, "net/netip": true, "net/mail": true, "mime": true, "regexp": true,
	"math/big": true, "unicode/utf8": true, "errors": true, "fmt": true, "go/format": true,
}

// Single funcs of packages that otherwise do I/O.
var inputErrorFuncs = map[string]map[string]bool{
	"time":          {"Parse": true, "ParseInLocation": true, "ParseDuration": true},
	"net":           {"ParseCIDR": true, "ParseMAC": true, "SplitHostPort": true},
	"path":          {"Match": true},
	"path/filepath": {"Match": true, "Rel": true},
}

// ErrorReturns lists the returns worth a test: errors the code makes itself (errors.New,
// fmt.Errorf, ErrNotFound, &ParseError{…}), and errors passed on from input-only calls like
// strconv.Atoi or a pure func of the same package. Errors passed on from I/O are left out,
// reaching them takes fakes that most code doesn't have, and listing them all buries the rest.
func (p *Package) ErrorReturns(fset *token.FileSet, file *ast.File, src []byte) []ErrorReturn {
	imports := newImportTable(file)
	var returns []ErrorReturn
	ast.Inspect(file, func(node ast.Node) bool {
		var fnType *ast.FuncType
		var body *ast.BlockStmt
		var receiverName, receiverType string
		switch node := node.(type) {
		case *ast.FuncDecl:
			fnType, body = node.Type, node.Body
			receiverName, receiverType = receiverOf(node)
		case *ast.FuncLit:
			fnType, body = node.Type, node.Body
		default:
			return true
		}
		if body == nil || !returnsError(fnType) {
			return true
		}

		fn := errorFunc{pkg: p, imports: imports, body: body, receiverName: receiverName, receiverType: receiverType}
		for _, ret := range ownReturns(body) {
			if fn.testable(ret.Results[len(ret.Results)-1], ret.Pos(), 0) {
				start, end := fset.Position(ret.Pos()), fset.Position(ret.End())
				returns = append(returns, ErrorReturn{
					Line:   start.Line,
					Column: start.Column,
					Code:   firstLine(string(src[start.Offset:end.Offset])),
				})
			}
		}
		return true
	})
	return returns
}

func returnsError(fnType *ast.FuncType) bool {
	results := fieldTypes(fnType.Results)
	if len(results) == 0 {
		return false
	}
	ident, ok := results[len(results)-1].(*ast.Ident)
	return ok && ident.Name == "error"
}

// ownReturns are the returns of body itself; those of nested func literals belong to them.
func ownReturns(body *ast.BlockStmt) []*ast.ReturnStmt {
	var returns []*ast.ReturnStmt
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(node.Results) > 0 {
				returns = append(returns, node)
			}
		}
		return true
	})
	return returns
}

type errorFunc struct {
	pkg                        *Package
	imports                    importTable
	body                       *ast.BlockStmt
	receiverName, receiverType string
}

// testable decides one error expression. before is where it's used, so the right assignment
// of a variable counts; depth stops err = otherErr chains from looping.
func (fn errorFunc) testable(expr ast.Expr, before token.Pos, depth int) bool {
	if depth > 4 {
		return false
	}
	switch expr := expr.(type) {
	case *ast.ParenExpr:
		return fn.testable(expr.X, before, depth+1)
	case *ast.Ident:
		if expr.Name == "nil" {
			return false
		}
		if value, ok := fn.lastAssignment(expr.Name, before); ok {
			return fn.testable(value, value.Pos(), depth+1)
		}
		// Not assigned here: a package-level ErrNotFound, or a param we can't see into.
		return isSentinelName(expr.Name)
	case *ast.SelectorExpr:
		// io.EOF, sql.ErrNoRows: the code decided to return that error.
		return isSentinelName(expr.Sel.Name) || expr.Sel.Name == "EOF"
	case *ast.UnaryExpr, *ast.CompositeLit:
		return true // &ParseError{...}
	case *ast.CallExpr:
		return fn.testableCall(expr, before, depth)
	default:
		return false
	}
}

// A call either wraps an error it's given (fmt.Errorf("…: %w", err), errors.Join(err)), which
// is as testable as that error, or makes a new one, or calls something that returns one.
func (fn errorFunc) testableCall(call *ast.CallExpr, before token.Pos, depth int) bool {
	if wrapped, ok := wrappedError(call); ok {
		return fn.testable(wrapped, before, depth+1)
	}
	if fn.imports.isMember(call.Fun, "errors", "New") || fn.imports.isMember(call.Fun, "fmt", "Errorf") {
		return true
	}
	return fn.inputOnlyCall(call)
}

// wrappedError is the first arg that looks like an error: named err, fooErr or errFoo, or a
// field like b.err.
func wrappedError(call *ast.CallExpr) (ast.Expr, bool) {
	for _, arg := range call.Args {
		switch arg := arg.(type) {
		case *ast.Ident:
			if isErrorVarName(arg.Name) {
				return arg, true
			}
		case *ast.SelectorExpr:
			if arg.Sel.Name == "err" || strings.HasSuffix(arg.Sel.Name, "Err") {
				return arg, true
			}
		}
	}
	return nil, false
}

// inputOnlyCall: strconv.Atoi, base64.StdEncoding.DecodeString, a pure func or method of this package.
func (fn errorFunc) inputOnlyCall(call *ast.CallExpr) bool {
	switch callee := call.Fun.(type) {
	case *ast.Ident:
		return fn.pkg.pureFunc(callee.Name)
	case *ast.SelectorExpr:
		if pkgPath, member, ok := fn.imports.resolve(callee); ok {
			return (inputErrorPackages[pkgPath] || inputErrorFuncs[pkgPath][member]) && !isImpureMember(pkgPath, member)
		}
		if inner, ok := callee.X.(*ast.SelectorExpr); ok {
			pkgPath, _, ok := fn.imports.resolve(inner)
			return ok && inputErrorPackages[pkgPath]
		}
		if ident, ok := callee.X.(*ast.Ident); ok && fn.receiverName != "" && ident.Name == fn.receiverName {
			return fn.pkg.pureFunc(fn.receiverType + "." + callee.Sel.Name)
		}
	}
	return false
}

// lastAssignment finds the value assigned to name last before pos: in `n, err := strconv.Atoi(s)`
// that's the call. Params and named results have no assignment and aren't followed.
func (fn errorFunc) lastAssignment(name string, before token.Pos) (ast.Expr, bool) {
	var value ast.Expr
	var at token.Pos
	consider := func(lhs []ast.Expr, rhs []ast.Expr, pos token.Pos) {
		if pos >= before || pos < at {
			return
		}
		for i, target := range lhs {
			ident, ok := target.(*ast.Ident)
			if !ok || ident.Name != name {
				continue
			}
			switch {
			case len(rhs) == len(lhs):
				value, at = rhs[i], pos
			case len(rhs) == 1:
				value, at = rhs[0], pos // n, err := f()
			}
		}
	}
	ast.Inspect(fn.body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			consider(node.Lhs, node.Rhs, node.Pos())
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(node.Names))
			for i, ident := range node.Names {
				lhs[i] = ident
			}
			consider(lhs, node.Values, node.Pos())
		}
		return true
	})
	return value, value != nil
}

// ErrNotFound, errEmpty: a package-level error the code returns on purpose.
func isSentinelName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Err")
	if !ok {
		rest, ok = strings.CutPrefix(name, "err")
	}
	return ok && rest != "" && startsUpper(rest)
}

func isErrorVarName(name string) bool {
	return name == "err" || strings.HasSuffix(name, "Err") || isSentinelName(name)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}
