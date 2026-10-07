package detect

import "go/ast"

func detectHTTPHandler(fn *ast.FuncDecl, imports importTable) (string, bool) {
	var reason string
	switch {
	case hasHandlerSignature(fn.Type, imports):
		reason = "handler signature"
	case returnsHandlerFunc(fn.Type, imports):
		reason = "returns http.HandlerFunc"
	default:
		return "", false
	}

	if decodesJSON(fn.Body, imports) {
		reason += ", decodes JSON body"
	}
	return reason, true
}

func hasHandlerSignature(fnType *ast.FuncType, imports importTable) bool {
	params := fieldTypes(fnType.Params)
	if len(params) != 2 {
		return false
	}

	writer, request := params[0], params[1]
	if !imports.isMember(writer, "net/http", "ResponseWriter") {
		return false
	}
	requestPtr, ok := request.(*ast.StarExpr)
	return ok && imports.isMember(requestPtr.X, "net/http", "Request")
}

func returnsHandlerFunc(fnType *ast.FuncType, imports importTable) bool {
	results := fieldTypes(fnType.Results)
	return len(results) == 1 && imports.isMember(results[0], "net/http", "HandlerFunc")
}

func decodesJSON(body *ast.BlockStmt, imports importTable) bool {
	return containsNode(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return false
		}
		return imports.isMember(call.Fun, "encoding/json", "NewDecoder") ||
			imports.isMember(call.Fun, "encoding/json", "Unmarshal")
	})
}
