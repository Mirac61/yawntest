package gen

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/Mirac61/yawntest/internal/detect"
)

var fuzzTemplate = template.Must(template.New("fuzz").Parse(`
// lazytest: pure-func / no panic, deterministic{{if .Invariant}}, {{.Invariant}}{{end}}
func FuzzLazytest_{{.Func}}(f *testing.F) {
{{- range .Seeds}}
	f.Add({{.}})
{{- end}}

	f.Fuzz(func(t *testing.T, {{.FuzzParams}}) {
		{{.NoPanic}}
{{- range .Setup}}
		{{.}}
{{- end}}
		{{.Got}} := {{.Call}}
		{{.Again}} := {{.Call}}

		// NaN never equals itself, so results that print the same count as equal.
{{- range .Compare}}
		if !reflect.DeepEqual({{.Got}}, {{.Again}}) && fmt.Sprint({{.Got}}) != fmt.Sprint({{.Again}}) {
			t.Errorf("{{$.Func}}({{$.ArgFormat}}) is not deterministic: got %v, then %v", {{$.Args}}, {{.Got}}, {{.Again}})
		}
{{- end}}
{{- if .PartsSumToTotal}}

		var sum {{.TotalType}}
		for _, part := range got {
			sum += part
		}
		if len(got) > 0 && sum != {{.Total}} {
			t.Errorf("{{.Func}}({{.ArgFormat}}) parts add up to %v, want the total %v", {{.Args}}, sum, {{.Total}})
		}
{{- end}}
{{- if .EndNotBeforeStart}}

		if got1.Before(got0) {
			t.Errorf("{{.Func}}({{.ArgFormat}}) ends at %v, before its start %v", {{.Args}}, got1, got0)
		}
{{- end}}
	})
}
`))

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

	var args []fuzzArg
	for i, param := range match.Params {
		args = append(args, b.fuzzArg(i, param))
	}

	seeds := seedRows(args)
	if match.Invariant == detect.InvariantPartsSumToTotal && len(args) == 2 {
		for _, pair := range splitSeeds {
			seeds = append(seeds, typed(args[0].fuzzType, pair[0])+", "+typed(args[1].fuzzType, pair[1]))
		}
	}
	got, again, compare := resultNames(len(match.Results))

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
		"NoPanic":    noPanic(""),
		"Seeds":      seeds,
		"FuzzParams": strings.Join(fuzzParams, ", "),
		"Setup":      setup,
		"Got":        got,
		"Again":      again,
		"Call":       match.Name + "(" + strings.Join(callArgs, ", ") + ")",
		"Compare":    compare,
		"ArgFormat":  strings.Join(formats, ", "),
		"Args":       strings.Join(argNames, ", "),

		"Invariant":         match.Invariant,
		"PartsSumToTotal":   match.Invariant == detect.InvariantPartsSumToTotal,
		"Total":             args[0].callArg,
		"TotalType":         match.Params[0].Type,
		"EndNotBeforeStart": match.Invariant == detect.InvariantEndNotBeforeStart,
	})

	summary := fmt.Sprintf("fuzz, %d seeds", len(seeds))
	if match.Invariant != "" {
		summary += ", " + string(match.Invariant)
	}
	return summary
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
		arg.seeds = values(unixCases)
		value = "time.Unix(" + fuzzName + ", 0).UTC()"
	} else {
		arg.seeds = values(b.edgeCases(elemType))
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
