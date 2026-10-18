package detect

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// Package is what detection knows about the other files of a directory: its funcs, methods,
// types and consts. It lets the rules look past one file, e.g. a pure func calling a helper
// that does I/O, or a ParseColor in parse.go whose constants live in color.go.
type Package struct {
	funcs  map[string][]*funcInfo // "Name" or "Type.Name"; several for build-constrained variants
	consts map[string][]string    // type name → typed consts, in source order
	basics map[string]string      // type name → basic type, for `type Slug string`
	vars   map[string]bool        // package-level vars

	uncomparableTypes map[string]bool   // type name → defined as a func, chan or interface
	namedAs           map[string]string // type Middleware Handler: Middleware → Handler
	files             []*ast.File

	// Set by SetTypes. Without it, or for a node the type checker couldn't resolve, the rules read
	// the syntax alone.
	path    string
	info    *types.Info
	program *Program
}

// Program links the packages of one scan by import path, so purity follows calls between them.
// load, if set, brings in a package of the module that the scan didn't cover, on first use.
type Program struct {
	byPath map[string]*Package
	load   func(path string) *Package
	tried  map[string]bool
}

func NewProgram(load func(path string) *Package) *Program {
	return &Program{byPath: map[string]*Package{}, load: load, tried: map[string]bool{}}
}

func (p *Program) lookup(path string) *Package {
	if pkg, ok := p.byPath[path]; ok || p.load == nil || p.tried[path] {
		return pkg
	}
	p.tried[path] = true
	pkg := p.load(path)
	if _, ok := p.byPath[path]; !ok && pkg != nil {
		// SetTypes didn't add it: the package didn't type-check. Keep the syntax-only package,
		// so every caller gets the same answer.
		p.byPath[path] = pkg
	}
	return pkg
}

// SetTypes gives the package its import path and type information, and adds it to program.
func (p *Package) SetTypes(path string, info *types.Info, program *Program) {
	p.path, p.info, p.program = path, info, program
	program.byPath[path] = p
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
	pkg := &Package{
		funcs: map[string][]*funcInfo{}, consts: map[string][]string{}, basics: map[string]string{}, vars: map[string]bool{},
		uncomparableTypes: map[string]bool{}, namedAs: map[string]string{}, files: files,
	}
	for _, file := range files {
		imports := newImportTable(file)
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				key := funcName(decl)
				pkg.funcs[key] = append(pkg.funcs[key], &funcInfo{decl: decl, imports: imports})
			case *ast.GenDecl:
				pkg.addConsts(decl)
				pkg.addBasics(decl)
				pkg.addVars(decl)
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

func (p *Package) addBasics(decl *ast.GenDecl) {
	if decl.Tok != token.TYPE {
		return
	}
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec)
		switch typ := typeSpec.Type.(type) {
		case *ast.Ident:
			if typeSpec.TypeParams == nil && basicTypes[typ.Name] {
				p.basics[typeSpec.Name.Name] = typ.Name
			}
			p.namedAs[typeSpec.Name.Name] = typ.Name
		case *ast.FuncType, *ast.ChanType, *ast.InterfaceType:
			p.uncomparableTypes[typeSpec.Name.Name] = true
		}
	}
}

// uncomparableNamed follows type Middleware Handler to Handler, whatever order they're declared in.
func (p *Package) uncomparableNamed(name string) bool {
	for seen := map[string]bool{}; !seen[name]; name = p.namedAs[name] {
		if p.uncomparableTypes[name] {
			return true
		}
		seen[name] = true
	}
	return false
}

// uncomparableResult reports a result of func, chan or non-error interface type. With type
// information that's exact; without, it sees literal types and the package's own named ones.
func (p *Package) uncomparableResult(fn *ast.FuncDecl, imports importTable) bool {
	for _, expr := range fieldTypes(fn.Type.Results) {
		if p.info != nil {
			if typed, ok := p.info.Types[expr]; ok && typed.Type != nil {
				if uncomparable(typed.Type) {
					return true
				}
				continue
			}
		}
		switch expr := expr.(type) {
		case *ast.FuncType, *ast.ChanType:
			return true
		case *ast.InterfaceType:
			return true
		case *ast.Ident:
			if p.uncomparableNamed(expr.Name) {
				return true
			}
		case *ast.SelectorExpr:
			if imports.isMember(expr, "net/http", "Handler") || imports.isMember(expr, "net/http", "HandlerFunc") {
				return true
			}
		}
	}
	return false
}

func uncomparable(typ types.Type) bool {
	switch typ.Underlying().(type) {
	case *types.Signature, *types.Chan:
		return true
	case *types.Interface:
		return !types.Identical(typ, types.Universe.Lookup("error").Type())
	default:
		return false
	}
}

// withUnderlying fills in Underlying for fields of a basic type of the package, as written
// without a package qualifier: gen converts to them in a test of the same package.
func (p *Package) withUnderlying(fields []Field) []Field {
	for i, field := range fields {
		if field.Type == "" {
			fields[i].Underlying = p.basics[field.Expr]
		}
	}
	return fields
}

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

// Without type information, calls into other packages of the module aren't followed.
func (p *Package) bodyImpure(fn *ast.FuncDecl, imports importTable) bool {
	if fn.Body == nil {
		return false // assembly, e.g. math kernels
	}
	receiverName, receiverType := receiverOf(fn)
	locals := localNames(fn)
	return containsNode(fn.Body, func(node ast.Node) bool {
		if p.writesPackageState(node, locals, imports) {
			return true
		}
		if impure, resolved := p.typedImpure(node); resolved {
			return impure
		}
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

// resolved is false when there is no type information for the node.
func (p *Package) typedImpure(node ast.Node) (impure, resolved bool) {
	var expr ast.Expr
	switch node := node.(type) {
	case *ast.CallExpr:
		expr = node.Fun
	case *ast.SelectorExpr:
		expr = node
	default:
		return false, false
	}
	switch object := p.objectOf(expr).(type) {
	case *types.Func:
		return p.funcImpure(object), true
	case *types.Var:
		// os.Stdout and friends; a local variable or a field is no I/O by itself.
		pkg := object.Pkg()
		return pkg != nil && object.Parent() == pkg.Scope() && ioPackages[pkg.Path()], true
	case nil:
		return false, false
	default:
		return false, true // types, consts, builtins
	}
}

func (p *Package) objectOf(expr ast.Expr) types.Object {
	if p.info == nil {
		return nil
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return p.info.Uses[expr]
	case *ast.SelectorExpr:
		if selection, ok := p.info.Selections[expr]; ok {
			return selection.Obj()
		}
		return p.info.Uses[expr.Sel]
	}
	return nil
}

func (p *Package) funcImpure(fn *types.Func) bool {
	pkg := fn.Pkg()
	if pkg == nil {
		return false // error.Error and other universe methods
	}
	path := pkg.Path()
	if abstractMethod(fn) {
		// Some implementation decides; for io.Writer or a Repository of the module that's I/O.
		return ioPackages[path] || !isStandard(path)
	}
	if target := p.packageAt(path); target != nil {
		return target.calledImpure(funcKey(fn))
	}
	if !isStandard(path) {
		return false // a dependency yawntest can't see into, as without type information
	}
	if isMethod(fn) {
		// time.Time.After is a comparison, not the timer time.After.
		return ioPackages[path] || impureMembers[path][fn.Name()]
	}
	return isImpureMember(path, fn.Name())
}

func (p *Package) packageAt(path string) *Package {
	if path == p.path {
		return p
	}
	if p.program != nil {
		return p.program.lookup(path)
	}
	return nil
}

// funcKey is the name funcs are indexed by: Name, or Type.Name for methods.
func funcKey(fn *types.Func) string {
	signature, ok := fn.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return fn.Name()
	}
	recv := signature.Recv().Type()
	if pointer, ok := recv.(*types.Pointer); ok {
		recv = pointer.Elem()
	}
	if named, ok := recv.(*types.Named); ok {
		return named.Obj().Name() + "." + fn.Name()
	}
	return fn.Name()
}

func isMethod(fn *types.Func) bool {
	signature, ok := fn.Type().(*types.Signature)
	return ok && signature.Recv() != nil
}

func abstractMethod(fn *types.Func) bool {
	signature, ok := fn.Type().(*types.Signature)
	return ok && signature.Recv() != nil && types.IsInterface(signature.Recv().Type())
}

// isStandard: standard library import paths have no dot in their first element.
func isStandard(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
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
