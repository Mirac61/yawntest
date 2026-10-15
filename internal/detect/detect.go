package detect

import (
	"go/ast"
	"go/token"
)

// env is what a detector sees besides the func: its file's imports and its package.
type env struct {
	imports importTable
	pkg     *Package
}

func (e env) hasSideEffects(fn *ast.FuncDecl) bool {
	return e.pkg.hasSideEffects(fn, e.imports)
}

type funcDetector func(fn *ast.FuncDecl, e env) (reason string, ok bool)

// First hit wins, so validation beats pure-func.
var funcDetectors = []struct {
	pattern Pattern
	detect  funcDetector
}{
	{PatternHTTPHandler, detectHTTPHandler},
	{PatternValidation, detectValidation},
	{PatternPureFunc, detectPureFunc},
}

// File detects one file as if it were its package's only file.
func File(fset *token.FileSet, file *ast.File) []Match {
	return NewPackage([]*ast.File{file}).File(fset, file)
}

// File returns the matches of one of the package's files, in source order.
func (p *Package) File(fset *token.FileSet, file *ast.File) []Match {
	e := env{imports: newImportTable(file), pkg: p}

	var matches []Match
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if match, ok := matchFunc(fset, decl, e); ok {
				matches = append(matches, match)
			}
			matches = append(matches, p.funcPairs(fset, decl, e)...)
		case *ast.GenDecl:
			for _, match := range matchJSONStructs(fset, decl, e.imports) {
				match.Fields = p.withUnderlying(match.Fields)
				matches = append(matches, match)
			}
		}
	}
	return matches
}

func matchFunc(fset *token.FileSet, fn *ast.FuncDecl, e env) (Match, bool) {
	imports := e.imports
	// No body: implemented in assembly.
	if fn.Body == nil {
		return Match{}, false
	}
	// Methods on generic types (T[K]) can't be called without a type argument.
	if fn.Recv != nil && receiverTypeName(fn.Recv.List[0].Type) == "" {
		return Match{}, false
	}

	for _, detector := range funcDetectors {
		reason, ok := detector.detect(fn, e)
		if !ok {
			continue
		}

		match := newMatch(fset, fn, funcName(fn), detector.pattern, reason)
		match.Params = e.pkg.withUnderlying(fields(fn.Type.Params, imports))
		match.Results = e.pkg.withUnderlying(fields(fn.Type.Results, imports))
		match.NamedValidator = detector.pattern == PatternValidation && validationPrefix(fn.Name.Name) != ""
		match.DecodesJSON = detector.pattern == PatternHTTPHandler && decodesJSON(fn.Body, imports)
		if detector.pattern == PatternPureFunc {
			match.Invariant = invariantOf(fn.Name.Name, match.Params, match.Results)
		}
		return match, true
	}
	return Match{}, false
}

func newMatch(fset *token.FileSet, node ast.Node, name string, pattern Pattern, reason string) Match {
	return Match{
		Line:    fset.Position(node.Pos()).Line,
		EndLine: fset.Position(node.End()).Line,
		Name:    name,
		Pattern: pattern,
		Reason:  reason,
	}
}
