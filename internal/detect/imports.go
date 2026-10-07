package detect

import (
	"go/ast"
	"regexp"
	"strconv"
	"strings"
)

type importTable map[string]string

var majorVersionSuffix = regexp.MustCompile(`^v[0-9]+$`)

func newImportTable(file *ast.File) importTable {
	imports := importTable{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		name := localName(spec, path)
		if name == "_" || name == "." {
			continue
		}
		imports[name] = path
	}
	return imports
}

// math/rand/v2 resolves to rand.
// ponytail: wrong if package name != dir name; go/packages fixes that.
func localName(spec *ast.ImportSpec, path string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}

	elements := strings.Split(path, "/")
	last := elements[len(elements)-1]
	if majorVersionSuffix.MatchString(last) && len(elements) > 1 {
		return elements[len(elements)-2]
	}
	return last
}

func (imports importTable) resolve(expr ast.Expr) (pkgPath, member string, ok bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	pkgPath, ok = imports[pkgIdent.Name]
	if !ok {
		return "", "", false
	}
	return pkgPath, selector.Sel.Name, true
}

func (imports importTable) isMember(expr ast.Expr, pkgPath, member string) bool {
	gotPath, gotMember, ok := imports.resolve(expr)
	return ok && gotPath == pkgPath && gotMember == member
}
