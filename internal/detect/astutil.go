package detect

import (
	"go/ast"
	"go/types"
)

var basicTypes = map[string]bool{
	"bool": true, "string": true, "byte": true, "rune": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

func fieldTypes(fields *ast.FieldList) []ast.Expr {
	if fields == nil {
		return nil
	}

	var types []ast.Expr
	for _, field := range fields.List {
		// Unnamed params like func(int) still count once.
		count := max(len(field.Names), 1)
		for range count {
			types = append(types, field.Type)
		}
	}
	return types
}

func fields(list *ast.FieldList, imports importTable) []Field {
	if list == nil {
		return nil
	}

	var result []Field
	for _, field := range list.List {
		typ := typeName(field.Type, imports)
		expr := types.ExprString(field.Type)
		if len(field.Names) == 0 {
			result = append(result, Field{Type: typ, Expr: expr})
		}
		for _, name := range field.Names {
			result = append(result, Field{Name: name.Name, Type: typ, Expr: expr})
		}
	}
	return result
}

func typeName(expr ast.Expr, imports importTable) string {
	switch typ := expr.(type) {
	case *ast.ArrayType:
		isSlice := typ.Len == nil
		elem := scalarTypeName(typ.Elt, imports)
		if !isSlice || elem == "" {
			return ""
		}
		return "[]" + elem
	case *ast.Ellipsis:
		elem := scalarTypeName(typ.Elt, imports)
		if elem == "" {
			return ""
		}
		return "..." + elem
	default:
		return scalarTypeName(expr, imports)
	}
}

func scalarTypeName(expr ast.Expr, imports importTable) string {
	if ident, ok := expr.(*ast.Ident); ok && basicTypes[ident.Name] {
		return ident.Name
	}
	if imports.isMember(expr, "time", "Time") {
		return "time.Time"
	}
	return ""
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}

	receiver := receiverTypeName(fn.Recv.List[0].Type)
	if receiver == "" {
		return fn.Name.Name
	}
	return receiver + "." + fn.Name.Name
}

func receiverTypeName(expr ast.Expr) string {
	if pointer, ok := expr.(*ast.StarExpr); ok {
		expr = pointer.X
	}

	switch generic := expr.(type) {
	case *ast.IndexExpr:
		expr = generic.X
	case *ast.IndexListExpr:
		expr = generic.X
	}

	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func containsNode(root ast.Node, match func(ast.Node) bool) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if !found && node != nil {
			found = match(node)
		}
		return !found
	})
	return found
}
