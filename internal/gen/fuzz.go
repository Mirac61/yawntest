package gen

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/Mirac61/lazytest/internal/detect"
)

var fuzzTemplate = template.Must(template.New("fuzz").Parse(`
// lazytest: pure-func / no panic, deterministic
func FuzzLazytest_{{.Func}}(f *testing.F) {
{{- range .Seeds}}
	f.Add({{.}})
{{- end}}

	f.Fuzz(func(t *testing.T, {{.FuzzParams}}) {
{{- range .Setup}}
		{{.}}
{{- end}}
		{{.Got}} := {{.Call}}
		{{.Again}} := {{.Call}}
{{- range .Compare}}
		if !lazytestSame({{.Got}}, {{.Again}}) {
			t.Errorf("{{$.Func}}({{$.ArgFormat}}) is not deterministic: got %v, then %v", {{$.Args}}, {{.Got}}, {{.Again}})
		}
{{- end}}
	})
}
`))

var sameTemplate = template.Must(template.New("same").Parse(`
// lazytestSame is reflect.DeepEqual, except that NaN equals NaN.
func lazytestSame(a, b any) bool {
	return reflect.DeepEqual(a, b) || fmt.Sprintf("%#v", a) == fmt.Sprintf("%#v", b)
}
`))

// Unix 0, leap day 2024-02-29 12:00 UTC, 9999-12-31 23:59:59 UTC, one second before 1970.
var unixSeeds = []string{"int64(0)", "int64(1709208000)", "int64(253402300799)", "int64(-1)"}

var untypedSeeds = map[string][]string{
	"string":  {`""`, `" "`, `"a"`, `strings.Repeat("x", 10000)`, `"ünïcødé"`, `"😀"`, `"\x00"`, `"' OR 1=1 --"`},
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

// Names a fuzz arg must not take: the closure's own names and the packages it uses.
var reservedNames = map[string]bool{
	"_": true, "t": true, "f": true, "got": true, "again": true,
	"fmt": true, "math": true, "reflect": true, "strings": true, "testing": true, "time": true,
}

// fuzzArg is one param of the function under test, mapped onto a type Go fuzzing supports.
type fuzzArg struct {
	name     string // name of the fuzz closure param
	fuzzType string
	seeds    []string
	setup    string // turns the fuzz value into the param, if needed
	callArg  string
}

func (b *builder) fuzz(match detect.Match) string {
	b.use("testing", "reflect", "fmt")
	b.needsSame = true

	var args []fuzzArg
	for i, param := range match.Params {
		args = append(args, b.fuzzArg(i, param))
	}

	seeds := seedRows(args)
	got, again, compare := resultNames(match.Results)

	var fuzzParams, setup, callArgs, argNames, formats []string
	for _, arg := range args {
		fuzzParams = append(fuzzParams, arg.name+" "+arg.fuzzType)
		if arg.setup != "" {
			setup = append(setup, arg.setup)
		}
		callArgs = append(callArgs, arg.callArg)
		argNames = append(argNames, arg.name)
		formats = append(formats, "%v")
	}

	b.execute(fuzzTemplate, map[string]any{
		"Func":       match.Name,
		"Seeds":      seeds,
		"FuzzParams": strings.Join(fuzzParams, ", "),
		"Setup":      setup,
		"Got":        got,
		"Again":      again,
		"Call":       match.Name + "(" + strings.Join(callArgs, ", ") + ")",
		"Compare":    compare,
		"ArgFormat":  strings.Join(formats, ", "),
		"Args":       strings.Join(argNames, ", "),
	})
	return fmt.Sprintf("fuzz, %d seeds", len(seeds))
}

type sliceKind int

const (
	notSlice sliceKind = iota
	slice
	variadic
)

// []byte stays whole because Go fuzzing supports it natively.
func splitSlice(typ string) (elemType string, kind sliceKind) {
	if typ == "[]byte" {
		return typ, notSlice
	}
	if elemType, ok := strings.CutPrefix(typ, "[]"); ok {
		return elemType, slice
	}
	if elemType, ok := strings.CutPrefix(typ, "..."); ok {
		return elemType, variadic
	}
	return typ, notSlice
}

func (b *builder) fuzzArg(index int, param detect.Field) fuzzArg {
	paramName := param.Name
	if paramName == "" || reservedNames[paramName] {
		paramName = fmt.Sprintf("arg%d", index)
	}
	elemType, kind := splitSlice(param.Type)

	fuzzName := paramName
	if kind != notSlice {
		fuzzName += "Elem"
	}

	arg := fuzzArg{fuzzType: elemType, callArg: paramName}
	value := fuzzName
	if elemType == "time.Time" {
		b.use("time")
		fuzzName += "Unix"
		arg.fuzzType = "int64"
		arg.seeds = unixSeeds
		value = "time.Unix(" + fuzzName + ", 0).UTC()"
	} else {
		arg.seeds = b.typedSeeds(elemType)
	}
	arg.name = fuzzName

	// ponytail: slices get one element only; nil, empty and huge slices need a table test.
	switch kind {
	case notSlice:
		if value != fuzzName {
			arg.setup = paramName + " := " + value
		}
	case slice:
		arg.setup = fmt.Sprintf("%s := []%s{%s}", paramName, elemType, value)
	case variadic:
		arg.setup = fmt.Sprintf("%s := []%s{%s}", paramName, elemType, value)
		arg.callArg = paramName + "..."
	}
	return arg
}

// f.Add needs the exact param type, so seeds get converted unless Go's default type already fits.
func (b *builder) typedSeeds(typ string) []string {
	needsConversion := typ != "string" && typ != "bool" && typ != "int" && typ != "float64" && typ != "[]byte"

	var seeds []string
	for _, seed := range untypedSeeds[typ] {
		if strings.Contains(seed, "math.") {
			b.use("math")
		}
		if strings.Contains(seed, "strings.") {
			b.use("strings")
		}

		if needsConversion {
			seed = typ + "(" + seed + ")"
		}
		seeds = append(seeds, seed)
	}
	return seeds
}

// seedRows combines the per-arg seeds into f.Add calls; shorter lists wrap around.
func seedRows(args []fuzzArg) []string {
	rowCount := 0
	for _, arg := range args {
		rowCount = max(rowCount, len(arg.seeds))
	}

	rows := make([]string, rowCount)
	for row := range rows {
		var values []string
		for _, arg := range args {
			values = append(values, arg.seeds[row%len(arg.seeds)])
		}
		rows[row] = strings.Join(values, ", ")
	}
	return rows
}

type resultPair struct {
	Got   string
	Again string
}

func resultNames(count int) (got, again string, compare []resultPair) {
	if count == 1 {
		return "got", "again", []resultPair{{Got: "got", Again: "again"}}
	}

	var gots, agains []string
	for i := range count {
		pair := resultPair{Got: fmt.Sprintf("got%d", i), Again: fmt.Sprintf("again%d", i)}
		gots = append(gots, pair.Got)
		agains = append(agains, pair.Again)
		compare = append(compare, pair)
	}
	return strings.Join(gots, ", "), strings.Join(agains, ", "), compare
}
