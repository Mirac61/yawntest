package detect

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

func (p *Package) addVars(decl *ast.GenDecl) {
	if decl.Tok != token.VAR {
		return
	}
	for _, spec := range decl.Specs {
		for _, name := range spec.(*ast.ValueSpec).Names {
			if name.Name != "_" {
				p.vars[name.Name] = true
			}
		}
	}
}

// writesPackageState reports whether node assigns to a package-level variable, of this
// package or an imported one: x = v, x++, x.f = v, x[k] = v, copy(x[:], v), clear(x).
// Such a func isn't deterministic, and a generated test calling it changes what the other
// tests of the package see. Calls on package vars, like a mutex's Lock or a cached regexp's
// MatchString, don't count: most of them only read.
func (p *Package) writesPackageState(node ast.Node, locals map[string]bool, imports importTable) bool {
	var targets []ast.Expr
	switch node := node.(type) {
	case *ast.AssignStmt:
		if node.Tok == token.DEFINE {
			return false
		}
		targets = node.Lhs
	case *ast.IncDecStmt:
		targets = []ast.Expr{node.X}
	case *ast.CallExpr:
		ident, ok := ast.Unparen(node.Fun).(*ast.Ident)
		if !ok || len(node.Args) == 0 || !p.isBuiltin(ident, "copy", "clear") {
			return false
		}
		targets = node.Args[:1]
	default:
		return false
	}
	for _, target := range targets {
		if p.packageVarAt(target, locals, imports) {
			return true
		}
	}
	return false
}

func (p *Package) isBuiltin(ident *ast.Ident, names ...string) bool {
	if object := p.objectOf(ident); object != nil {
		_, ok := object.(*types.Builtin)
		return ok && slices.Contains(names, ident.Name)
	}
	return slices.Contains(names, ident.Name) && len(p.funcs[ident.Name]) == 0
}

// packageVarAt follows x.f[i][:] down to x and reports whether x is a package-level var.
func (p *Package) packageVarAt(expr ast.Expr, locals map[string]bool, imports importTable) bool {
	for {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.SliceExpr:
			expr = e.X
		case *ast.StarExpr:
			expr = e.X
		case *ast.SelectorExpr:
			if object := p.objectOf(e); object != nil {
				if v, ok := object.(*types.Var); ok && packageLevel(v) {
					return true // pkg.Var
				}
			} else if _, _, ok := imports.resolve(e); ok {
				return true // pkg.Name can only be assigned when it's a var
			}
			expr = e.X
		case *ast.Ident:
			if object := p.objectOf(e); object != nil {
				v, ok := object.(*types.Var)
				return ok && packageLevel(v)
			}
			return p.vars[e.Name] && !locals[e.Name]
		default:
			return false // a call's result, a composite literal: nothing that outlives the call
		}
	}
}

func packageLevel(v *types.Var) bool {
	return v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

// localNames collects every name fn declares, for when there is no type information.
// Scopes are ignored: a local anywhere in fn hides the package var everywhere in it,
// so a shadowed name never makes a pure func impure.
func localNames(fn *ast.FuncDecl) map[string]bool {
	locals := map[string]bool{}
	addFields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				locals[name.Name] = true
			}
		}
	}
	addFields(fn.Recv)
	addFields(fn.Type.Params)
	addFields(fn.Type.Results)
	if fn.Body == nil {
		return locals
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						locals[ident.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				for _, expr := range []ast.Expr{node.Key, node.Value} {
					if ident, ok := expr.(*ast.Ident); ok {
						locals[ident.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, name := range node.Names {
				locals[name.Name] = true
			}
		case *ast.FuncLit:
			addFields(node.Type.Params)
			addFields(node.Type.Results)
		}
		return true
	})
	return locals
}
