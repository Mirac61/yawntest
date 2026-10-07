package detect

import (
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

func matchJSONStructs(fset *token.FileSet, decl *ast.GenDecl) []Match {
	if decl.Tok != token.TYPE {
		return nil
	}

	var matches []Match
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec)
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok || !typeSpec.Name.IsExported() {
			continue
		}
		// Generic structs need a type argument before they can be encoded.
		if typeSpec.TypeParams != nil {
			continue
		}
		if !hasJSONField(structType) {
			continue
		}

		match := newMatch(fset, typeSpec, typeSpec.Name.Name, PatternJSONRoundtrip, "has json tags")
		matches = append(matches, match)
	}
	return matches
}

func hasJSONField(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if hasJSONName(field) {
			return true
		}
	}
	return false
}

func hasJSONName(field *ast.Field) bool {
	if field.Tag == nil {
		return false
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return false
	}

	value, ok := reflect.StructTag(tag).Lookup("json")
	if !ok {
		return false
	}
	name, _, _ := strings.Cut(value, ",")
	return name != "-"
}
