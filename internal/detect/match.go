package detect

import "strings"

type Pattern string

const (
	PatternHTTPHandler   Pattern = "http-handler"
	PatternJSONRoundtrip Pattern = "json-roundtrip"
	PatternPureFunc      Pattern = "pure-func"
	PatternValidation    Pattern = "validation"
)

type Field struct {
	Name string
	Type string // basic type, "time.Time", "[]T" or "...T"; empty if gen can't build a value
	Expr string // the type as written in the source, e.g. "*User" or "error"
}

type Match struct {
	Line    int
	Name    string // Func, Type.Method or Type
	Pattern Pattern
	Reason  string

	Fields  []Field // structs: fields encoding/json writes
	Params  []Field // funcs
	Results []Field // funcs

	// Validate*, IsValid* or Check*: the name promises that empty input is rejected.
	NamedValidator bool
}

func (m Match) Symbol() string {
	_, method, isMethod := strings.Cut(m.Name, ".")
	if isMethod {
		return method
	}
	return m.Name
}
