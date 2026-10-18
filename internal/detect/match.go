package detect

import "strings"

type Pattern string

const (
	PatternHTTPHandler   Pattern = "http-handler"
	PatternJSONRoundtrip Pattern = "json-roundtrip"
	PatternPureFunc      Pattern = "pure-func"
	PatternValidation    Pattern = "validation"
	PatternRoundtrip     Pattern = "roundtrip"
	PatternEnumRoundtrip Pattern = "enum-roundtrip"
)

type Field struct {
	Name       string
	Type       string // basic type, "time.Time", "[]T" or "...T"; empty if gen can't build a value
	Expr       string // the type as written in the source, e.g. "*User" or "error"
	Underlying string // for a type of the package like `type Slug string`: "string"
}

// Basic is the basic type gen builds values of: Type, or else Underlying, converted to Expr.
func (f Field) Basic() string {
	if f.Type != "" {
		return f.Type
	}
	return f.Underlying
}

type Match struct {
	Line    int
	EndLine int
	Name    string // Func, Type.Method or Type
	Pattern Pattern
	Reason  string

	Fields  []Field // structs: fields encoding/json writes
	Params  []Field // funcs
	Results []Field // funcs

	// Validate*, IsValid* or Check*: the name promises that empty input is rejected.
	NamedValidator bool
	DecodesJSON    bool      // http handlers that read a JSON request body
	Invariant      Invariant // pure funcs: an extra property the fuzz test checks

	// roundtrip: the decode func (ParsePrice for FormatPrice) and its results.
	// enum-roundtrip: the enum type (Color for ParseColor).
	Partner        string
	PartnerResults []Field
	Consts         []string // enum-roundtrip: the consts that must parse back
}

func (m Match) Symbol() string {
	_, method, isMethod := strings.Cut(m.Name, ".")
	if isMethod {
		return method
	}
	return m.Name
}
