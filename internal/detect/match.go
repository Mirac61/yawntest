package detect

import "strings"

type Pattern string

const (
	PatternHTTPHandler   Pattern = "http-handler"
	PatternJSONRoundtrip Pattern = "json-roundtrip"
	PatternPureFunc      Pattern = "pure-func"
	PatternValidation    Pattern = "validation"
)

type Match struct {
	File    string
	Line    int
	Name    string // Func, Type.Method or Type
	Pattern Pattern
	Reason  string
}

func (m Match) Symbol() string {
	_, method, isMethod := strings.Cut(m.Name, ".")
	if isMethod {
		return method
	}
	return m.Name
}
