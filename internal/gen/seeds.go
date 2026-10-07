package gen

import "strings"

type edgeCase struct {
	Name  string
	Value string // Go expression of the param's exact type
}

var stringCases = []edgeCase{
	{Name: "empty", Value: `""`},
	{Name: "space", Value: `" "`},
	{Name: "single char", Value: `"a"`},
	{Name: "very long", Value: `strings.Repeat("x", 10000)`},
	{Name: "unicode", Value: `"ünïcødé"`},
	{Name: "emoji", Value: `"😀"`},
	{Name: "null byte", Value: `"\x00"`},
	{Name: "sql injection", Value: `"' OR 1=1 --"`},
}

// Unix seconds, because Go fuzzing can't take time.Time directly.
var unixCases = []edgeCase{
	{Name: "unix 0", Value: "int64(0)"},
	{Name: "leap day", Value: "int64(1709208000)"},    // 2024-02-29 12:00 UTC
	{Name: "year 9999", Value: "int64(253402300799)"}, // 9999-12-31 23:59:59 UTC
	{Name: "before 1970", Value: "int64(-1)"},
	{Name: "midnight", Value: "int64(1767225600)"},               // 2026-01-01 00:00:00 UTC
	{Name: "second before midnight", Value: "int64(1767225599)"}, // 2025-12-31 23:59:59 UTC
	{Name: "dst starts in berlin", Value: "int64(1774746000)"},   // 2026-03-29 02:00 CET jumps to 03:00 CEST
	{Name: "dst ends in berlin", Value: "int64(1792890000)"},     // 2026-10-25 03:00 CEST falls back to 02:00 CET
}

var untypedSeeds = map[string][]string{
	"bool":    {"false", "true"},
	"float64": {"0.0", "math.Copysign(0, -1)", "0.1", "-1.0", "1e308", "math.NaN()", "math.Inf(1)"},
	"float32": {"0", "math.Copysign(0, -1)", "0.1", "-1", "math.MaxFloat32", "math.NaN()", "math.Inf(1)"},
	"[]byte":  {"[]byte(nil)", "[]byte{}", `[]byte("a")`, "make([]byte, 10000)"},
	"int":     signedSeeds(""),
	"int8":    signedSeeds("8"),
	"int16":   signedSeeds("16"),
	"int32":   signedSeeds("32"),
	"rune":    signedSeeds("32"),
	"int64":   signedSeeds("64"),
	"uint":    unsignedSeeds(""),
	"uint8":   unsignedSeeds("8"),
	"byte":    unsignedSeeds("8"),
	"uint16":  unsignedSeeds("16"),
	"uint32":  unsignedSeeds("32"),
	"uint64":  unsignedSeeds("64"),
}

func signedSeeds(bits string) []string {
	return []string{"0", "1", "-1", "math.MaxInt" + bits, "math.MinInt" + bits}
}

func unsignedSeeds(bits string) []string {
	return []string{"0", "1", "math.MaxUint" + bits}
}

// edgeCases returns the edge values of a basic type. Seeds get converted to typ
// unless Go's default type for the constant already fits, because f.Add needs exact types.
func (b *builder) edgeCases(typ string) []edgeCase {
	if typ == "string" {
		b.use("strings")
		return stringCases
	}

	var cases []edgeCase
	for _, seed := range untypedSeeds[typ] {
		if strings.Contains(seed, "math.") {
			b.use("math")
		}
		cases = append(cases, edgeCase{Name: seed, Value: typed(typ, seed)})
	}
	return cases
}

func typed(typ, constant string) string {
	needsConversion := typ != "bool" && typ != "int" && typ != "float64" && typ != "[]byte"
	if needsConversion {
		return typ + "(" + constant + ")"
	}
	return constant
}

// Totals that don't divide evenly, so a split that drops the remainder fails.
var splitSeeds = [][2]string{{"100", "3"}, {"101", "2"}, {"1", "3"}, {"999", "7"}}

func values(cases []edgeCase) []string {
	var result []string
	for _, edge := range cases {
		result = append(result, edge.Value)
	}
	return result
}
