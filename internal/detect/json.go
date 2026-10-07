package detect

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"
)

func matchJSONStructs(fset *token.FileSet, decl *ast.GenDecl, imports importTable) []Match {
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
		match.Fields = encodedFields(structType, imports)
		matches = append(matches, match)
	}
	return matches
}

func hasJSONField(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		name, tagged := jsonTagName(field)
		if tagged && name != "-" {
			return true
		}
	}
	return false
}

// Embedded fields are left out, their zero value round-trips anyway.
func encodedFields(structType *ast.StructType, imports importTable) []Field {
	var encoded []Field
	for _, field := range structType.Fields.List {
		name, _ := jsonTagName(field)
		if name == "-" {
			continue
		}

		typ := typeName(field.Type, imports)
		expr := types.ExprString(field.Type)
		for _, fieldName := range field.Names {
			if fieldName.IsExported() {
				encoded = append(encoded, Field{Name: fieldName.Name, Type: typ, Expr: expr})
			}
		}
	}
	return encoded
}

func jsonTagName(field *ast.Field) (name string, tagged bool) {
	if field.Tag == nil {
		return "", false
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false
	}

	value, ok := reflect.StructTag(tag).Lookup("json")
	if !ok {
		return "", false
	}
	name, _, _ = strings.Cut(value, ",")
	return name, true
}
