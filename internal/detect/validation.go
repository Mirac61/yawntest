package detect

import (
	"go/ast"
	"strings"
	"unicode"
	"unicode/utf8"
)

var validationPrefixes = []string{"Validate", "IsValid", "Check"}

func detectValidation(fn *ast.FuncDecl, imports importTable) (string, bool) {
	result, ok := validationResult(fn.Type)
	if !ok {
		return "", false
	}
	// The generated test calls it with "a", " " and the like; Delete(path string) error must not run.
	if hasSideEffects(fn.Body, imports) {
		return "", false
	}
	params := fieldTypes(fn.Type.Params)

	prefix := validationPrefix(fn.Name.Name)
	if prefix != "" && hasSingleInput(fn, params) {
		return "name " + prefix + "*, returns " + result, true
	}
	// Only string here: Save(u *User) error has the same shape but validates nothing.
	if len(params) == 1 && isString(params[0]) {
		return "string param, returns " + result, true
	}
	return "", false
}

func validationResult(fnType *ast.FuncType) (string, bool) {
	results := fieldTypes(fnType.Results)
	if len(results) != 1 {
		return "", false
	}

	ident, ok := results[0].(*ast.Ident)
	if !ok {
		return "", false
	}
	if ident.Name != "error" && ident.Name != "bool" {
		return "", false
	}
	return ident.Name, true
}

func hasSingleInput(fn *ast.FuncDecl, params []ast.Expr) bool {
	switch len(params) {
	case 0:
		return fn.Recv != nil
	case 1:
		return true
	default:
		return false
	}
}

// checkName matches, Checkout does not.
func validationPrefix(name string) string {
	capitalized := capitalize(name)
	for _, prefix := range validationPrefixes {
		rest, found := strings.CutPrefix(capitalized, prefix)
		if !found {
			continue
		}
		if rest == "" || startsUpper(rest) {
			return prefix
		}
	}
	return ""
}

func isString(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "string"
}

func capitalize(s string) string {
	first, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(first)) + s[size:]
}

func startsUpper(s string) bool {
	first, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(first)
}
