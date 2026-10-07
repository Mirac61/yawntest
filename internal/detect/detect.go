package detect

import (
	"go/ast"
	"go/token"
)

type funcDetector func(fn *ast.FuncDecl, imports importTable) (reason string, ok bool)

// First hit wins, so validation beats pure-func.
var funcDetectors = []struct {
	pattern Pattern
	detect  funcDetector
}{
	{PatternHTTPHandler, detectHTTPHandler},
	{PatternValidation, detectValidation},
	{PatternPureFunc, detectPureFunc},
}

func File(fset *token.FileSet, file *ast.File) []Match {
	imports := newImportTable(file)

	var matches []Match
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if match, ok := matchFunc(fset, decl, imports); ok {
				matches = append(matches, match)
			}
		case *ast.GenDecl:
			matches = append(matches, matchJSONStructs(fset, decl, imports)...)
		}
	}
	return matches
}

func matchFunc(fset *token.FileSet, fn *ast.FuncDecl, imports importTable) (Match, bool) {
	// No body: implemented in assembly.
	if fn.Body == nil {
		return Match{}, false
	}
	// Methods on generic types (T[K]) can't be called without a type argument.
	if fn.Recv != nil && receiverTypeName(fn.Recv.List[0].Type) == "" {
		return Match{}, false
	}

	for _, detector := range funcDetectors {
		reason, ok := detector.detect(fn, imports)
		if !ok {
			continue
		}

		match := newMatch(fset, fn, funcName(fn), detector.pattern, reason)
		match.Params = fields(fn.Type.Params, imports)
		match.Results = fields(fn.Type.Results, imports)
		return match, true
	}
	return Match{}, false
}

func newMatch(fset *token.FileSet, node ast.Node, name string, pattern Pattern, reason string) Match {
	return Match{
		Line:    fset.Position(node.Pos()).Line,
		Name:    name,
		Pattern: pattern,
		Reason:  reason,
	}
}
