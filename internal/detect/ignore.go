package detect

import (
	"go/ast"
	"go/token"
	"strings"
)

// IgnoreDirective silences yawntest where it's written. In the doc comment of a func or type it
// covers the whole declaration: no candidate, no hints, no error paths. Anywhere else it covers
// its own line and the next one. Text after the directive is free for a reason.
const IgnoreDirective = "//yawntest:ignore"

// Ignored reports for a line whether a directive covers it.
type Ignored func(line int) bool

type lineRange struct{ start, end int }

func Ignores(fset *token.FileSet, file *ast.File) Ignored {
	docs := map[*ast.CommentGroup]ast.Node{}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Doc != nil {
				docs[decl.Doc] = decl
			}
		case *ast.GenDecl:
			if decl.Doc != nil {
				docs[decl.Doc] = decl
			}
			for _, spec := range decl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Doc != nil {
					docs[typeSpec.Doc] = typeSpec
				}
			}
		}
	}

	var ranges []lineRange
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if !isIgnoreDirective(comment.Text) {
				continue
			}
			line := fset.Position(comment.Pos()).Line
			if decl, ok := docs[group]; ok {
				ranges = append(ranges, lineRange{line, fset.Position(decl.End()).Line})
			} else {
				ranges = append(ranges, lineRange{line, line + 1})
			}
		}
	}

	return func(line int) bool {
		for _, r := range ranges {
			if r.start <= line && line <= r.end {
				return true
			}
		}
		return false
	}
}

func isIgnoreDirective(text string) bool {
	rest, ok := strings.CutPrefix(text, IgnoreDirective)
	return ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t')
}
