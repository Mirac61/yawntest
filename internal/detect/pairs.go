package detect

import (
	"go/ast"
	"go/token"
	"strings"
)

// Encode/decode pairs whose composition must give the input back. The prefixes are matched at a
// word boundary and the rest of the name has to agree: FormatPrice pairs with ParsePrice.
var roundtripPrefixes = []struct{ encode, decode string }{
	{"Encode", "Decode"},
	{"Marshal", "Unmarshal"},
	{"Format", "Parse"},
	{"Escape", "Unescape"},
	{"Quote", "Unquote"},
	{"Compress", "Decompress"},
	{"Serialize", "Deserialize"},
	{"To", "From"},
}

// Inputs a roundtrip is fuzzed with. Floats and times are left out: rounding and monotonic
// clock readings make "same value back" a matter of taste, not a bug.
var roundtripInputTypes = map[string]bool{
	"string": true, "[]byte": true, "bool": true, "byte": true, "rune": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
}

// Consts named like these usually stand for "no value" or a bound, and often don't parse back.
var enumSentinelWords = map[string]bool{
	"unknown": true, "invalid": true, "undefined": true, "unspecified": true, "unset": true,
	"max": true, "count": true, "num": true, "sentinel": true,
}

// funcPairs returns the matches that need a second declaration of the package: the decode half
// of a roundtrip, or the String method and consts of an enum.
func (p *Package) funcPairs(fset *token.FileSet, fn *ast.FuncDecl, e env) []Match {
	if !isPlainExported(fn) || fn.Body == nil {
		return nil
	}
	var matches []Match
	if match, ok := p.roundtrip(fset, fn, e); ok {
		matches = append(matches, match)
	}
	if match, ok := p.enumRoundtrip(fset, fn, e); ok {
		matches = append(matches, match)
	}
	return matches
}

func isPlainExported(fn *ast.FuncDecl) bool {
	return fn.Recv == nil && fn.Name.IsExported() && fn.Type.TypeParams == nil
}

func (p *Package) roundtrip(fset *token.FileSet, encode *ast.FuncDecl, e env) (Match, bool) {
	decodeName, ok := decodeNameFor(encode.Name.Name)
	if !ok || len(p.funcs[decodeName]) == 0 {
		return Match{}, false
	}
	decode := p.funcs[decodeName][0]
	if !isPlainExported(decode.decl) || decode.decl.Body == nil {
		return Match{}, false
	}

	encodeParams, encodeResults := fields(encode.Type.Params, e.imports), fields(encode.Type.Results, e.imports)
	decodeParams, decodeResults := fields(decode.decl.Type.Params, decode.imports), fields(decode.decl.Type.Results, decode.imports)
	if len(encodeParams) != 1 || len(decodeParams) != 1 {
		return Match{}, false
	}
	input := encodeParams[0]
	if !roundtripInputTypes[input.Type] {
		return Match{}, false
	}
	encoded, encodeOK := valueAndError(encodeResults)
	decoded, decodeOK := valueAndError(decodeResults)
	if !encodeOK || !decodeOK || encoded.Expr != decodeParams[0].Expr || decoded.Expr != input.Expr {
		return Match{}, false
	}
	if e.hasSideEffects(encode) || !p.pureFunc(decodeName) {
		return Match{}, false
	}

	match := newMatch(fset, encode, encode.Name.Name, PatternRoundtrip, "pairs with "+decodeName+", no I/O")
	match.Partner = decodeName
	match.Params = encodeParams
	match.Results = encodeResults
	match.PartnerResults = decodeResults
	return match, true
}

func decodeNameFor(name string) (string, bool) {
	for _, pair := range roundtripPrefixes {
		rest, found := strings.CutPrefix(name, pair.encode)
		if found && (rest == "" || startsUpper(rest)) {
			return pair.decode + rest, true
		}
	}
	return "", false
}

// valueAndError accepts (T) and (T, error).
func valueAndError(results []Field) (Field, bool) {
	switch {
	case len(results) == 1 && results[0].Expr != "error":
		return results[0], true
	case len(results) == 2 && results[1].Expr == "error":
		return results[0], true
	default:
		return Field{}, false
	}
}

// enumRoundtrip matches ParseColor(s string) (Color, error) when Color has a String method and
// typed consts: ParseColor(c.String()) must give back c for each of them.
func (p *Package) enumRoundtrip(fset *token.FileSet, parse *ast.FuncDecl, e env) (Match, bool) {
	if validationPrefix(parse.Name.Name) != "" || !strings.HasPrefix(parse.Name.Name, "Parse") {
		return Match{}, false
	}
	params, results := fields(parse.Type.Params, e.imports), fields(parse.Type.Results, e.imports)
	if len(params) != 1 || params[0].Type != "string" || len(results) != 2 || results[1].Expr != "error" {
		return Match{}, false
	}
	typeName := results[0].Expr
	if !p.hasValueStringer(typeName) {
		return Match{}, false
	}

	var consts []string
	for _, name := range p.consts[typeName] {
		if !hasWord(name, enumSentinelWords) {
			consts = append(consts, name)
		}
	}
	if len(consts) == 0 || e.hasSideEffects(parse) || !p.pureFunc(typeName+".String") {
		return Match{}, false
	}

	match := newMatch(fset, parse, parse.Name.Name, PatternEnumRoundtrip, typeName+" has String and consts")
	match.Partner = typeName
	match.Consts = consts
	match.Params = params
	match.Results = results
	return match, true
}

// The String method must have a value receiver, or c.String() on a const doesn't compile.
func (p *Package) hasValueStringer(typeName string) bool {
	infos := p.funcs[typeName+".String"]
	if len(infos) == 0 {
		return false
	}
	method := infos[0].decl
	if _, isPointer := method.Recv.List[0].Type.(*ast.StarExpr); isPointer {
		return false
	}
	results := fieldTypes(method.Type.Results)
	return len(fieldTypes(method.Type.Params)) == 0 && len(results) == 1 && isString(results[0])
}
