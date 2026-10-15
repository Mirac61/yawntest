package detect

import (
	"go/ast"
	"go/token"
)

// Package is what detection knows about the other files of a directory: its funcs, methods,
// types and consts. It lets the rules look past one file, e.g. a pure func calling a helper
// that does I/O, or a ParseColor in parse.go whose constants live in color.go.
type Package struct {
	funcs  map[string][]*funcInfo // "Name" or "Type.Name"; several for build-constrained variants
	consts map[string][]string    // type name → typed consts, in source order
	files  []*ast.File
}

type purity int

const (
	unknown purity = iota
	visiting
	pure
	impure
)

type funcInfo struct {
	decl    *ast.FuncDecl
	imports importTable
	state   purity
}

// NewPackage indexes the non-test files of one directory, generated files included: their
// funcs don't become candidates, but a generated helper doing I/O still makes its callers impure.
func NewPackage(files []*ast.File) *Package {
	pkg := &Package{funcs: map[string][]*funcInfo{}, consts: map[string][]string{}, files: files}
	for _, file := range files {
		imports := newImportTable(file)
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				key := funcName(decl)
				pkg.funcs[key] = append(pkg.funcs[key], &funcInfo{decl: decl, imports: imports})
			case *ast.GenDecl:
				pkg.addConsts(decl)
			}
		}
	}
	return pkg
}

// addConsts records typed consts, including the implicit repetition in iota blocks:
// const ( Red Color = iota; Green; Blue ) gives Color → Red, Green, Blue.
func (p *Package) addConsts(decl *ast.GenDecl) {
	if decl.Tok != token.CONST {
		return
	}
	var typeName string
	for _, spec := range decl.Specs {
		valueSpec := spec.(*ast.ValueSpec)
		switch {
		case valueSpec.Type != nil:
			typeName = ""
			if ident, ok := valueSpec.Type.(*ast.Ident); ok {
				typeName = ident.Name
			}
		case len(valueSpec.Values) > 0:
			typeName = ""
		}
		if typeName == "" {
			continue
		}
		for _, name := range valueSpec.Names {
			if name.Name != "_" {
				p.consts[typeName] = append(p.consts[typeName], name.Name)
			}
		}
	}
}

// hasSideEffects reports whether fn, or a func or method of this package it calls, does I/O.
func (p *Package) hasSideEffects(fn *ast.FuncDecl, imports importTable) bool {
	for _, info := range p.funcs[funcName(fn)] {
		if info.decl == fn {
			return p.impure(info)
		}
	}
	// Not indexed, e.g. a file detected on its own: judge the body alone.
	return p.bodyImpure(fn, imports)
}

// pureFunc reports whether every variant of the named package func or method is free of I/O.
// An unknown name is not pure: it's a local closure or something detection can't see.
func (p *Package) pureFunc(name string) bool {
	infos := p.funcs[name]
	if len(infos) == 0 {
		return false
	}
	for _, info := range infos {
		if p.impure(info) {
			return false
		}
	}
	return true
}

// Recursion counts as pure while it's being decided: a cycle alone does no I/O.
func (p *Package) impure(info *funcInfo) bool {
	switch info.state {
	case pure, visiting:
		return false
	case impure:
		return true
	}
	info.state = visiting
	if p.bodyImpure(info.decl, info.imports) {
		info.state = impure
		return true
	}
	info.state = pure
	return false
}

// ponytail: calls into other packages of the module aren't followed; needs go/types.
func (p *Package) bodyImpure(fn *ast.FuncDecl, imports importTable) bool {
	if fn.Body == nil {
		return false // assembly, e.g. math kernels
	}
	receiverName, receiverType := receiverOf(fn)
	return containsNode(fn.Body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.GoStmt:
			return true
		case *ast.SelectorExpr:
			if pkgPath, member, ok := imports.resolve(node); ok {
				return isImpureMember(pkgPath, member)
			}
			if ident, ok := node.X.(*ast.Ident); ok && receiverName != "" && ident.Name == receiverName {
				return p.calledImpure(receiverType + "." + node.Sel.Name)
			}
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok {
				return p.calledImpure(ident.Name)
			}
		}
		return false
	})
}

// calledImpure is false for names that aren't package funcs: builtins, closures, fields.
func (p *Package) calledImpure(name string) bool {
	for _, info := range p.funcs[name] {
		if p.impure(info) {
			return true
		}
	}
	return false
}

func receiverOf(fn *ast.FuncDecl) (name, typeName string) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", ""
	}
	field := fn.Recv.List[0]
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	return name, receiverTypeName(field.Type)
}
