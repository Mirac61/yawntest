package detect

import "strings"

type Pattern string

const (
	PatternHTTPHandler   Pattern = "http-handler"
	PatternJSONRoundtrip Pattern = "json-roundtrip"
	PatternPureFunc      Pattern = "pure-func"
	PatternValidation    Pattern = "validation"
)

// Type is a basic type, "time.Time", "[]T" or "...T". Empty means gen can't build a value.
type Field struct {
	Name string
	Type string
}

type Match struct {
	Line    int
	Name    string // Func, Type.Method or Type
	Pattern Pattern
	Reason  string

	Fields  []Field // structs: fields encoding/json writes
	Params  []Field // funcs
	Results int     // funcs
}

func (m Match) Symbol() string {
	_, method, isMethod := strings.Cut(m.Name, ".")
	if isMethod {
		return method
	}
	return m.Name
}
