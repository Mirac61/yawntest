package gen

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/Mirac61/yawntest/internal/detect"
)

var validationTemplate = template.Must(template.New("validation").Parse(`
// yawntest: validation / no panic; expected results are up to you
func TestYawntest_{{.TestName}}(t *testing.T) {
	tests := []struct {
		name  string
		input {{.InputType}}
	}{
{{- range .Cases}}
		{name: {{printf "%q" .Name}}{{if .Value}}, input: {{.Value}}{{end}}},
{{- end}}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			{{.NoPanic}}
			{{.Result}} := {{.Call}}(test.input)
			t.Skipf("TODO(yawntest): expected result for %s; got %v", test.name, {{.Result}})
		})
	}
}
`))

var zeroReceiverTemplate = template.Must(template.New("zeroReceiver").Parse(`
// yawntest: validation / no panic on the zero value; expected result is up to you
func TestYawntest_{{.TestName}}_ZeroValue(t *testing.T) {
	{{.NoPanic}}
	{{.Result}} := {{.Call}}()
	t.Skipf("TODO(yawntest): is the zero value valid? got %v", {{.Result}})
}
`))

var rejectsEmptyTemplate = template.Must(template.New("rejectsEmpty").Parse(`
// yawntest: validation / empty input is rejected
func TestYawntest_{{.TestName}}_RejectsEmpty(t *testing.T) {
	{{.NoPanic}}
{{- if eq .Result "err"}}
	if err := {{.Call}}(""); err == nil {
		t.Error("{{.Name}}(\"\") returned nil, want an error for empty input")
	}
{{- else}}
	if {{.Call}}("") {
		t.Error("{{.Name}}(\"\") returned true, want false for empty input")
	}
{{- end}}
}
`))

var localTypeExpr = regexp.MustCompile(`^\*?[A-Za-z_][A-Za-z0-9_]*$`)

func (b *builder) validation(match detect.Match) (summary string, ok bool) {
	b.use("testing")
	data := map[string]any{
		"Name":     match.Name,
		"TestName": TestName(match.Name),
		"Call":     callable(match.Name),
		"Result":   resultVar(match.Results[0].Expr),
		"NoPanic":  noPanic(zeroReceiverNote(match.Name)),
	}

	if len(match.Params) == 0 {
		b.execute(zeroReceiverTemplate, data)
		return "1 case (expected value TODO)", true
	}

	param := match.Params[0]
	inputType, cases, ok := b.validationInput(param, exported(funcPart(match.Name)))
	if !ok {
		return "", false
	}
	data["InputType"] = inputType
	data["Cases"] = cases
	b.execute(validationTemplate, data)

	caseCount := len(cases)
	if match.NamedValidator && param.Basic() == "string" {
		b.execute(rejectsEmptyTemplate, data)
		caseCount++
	}
	return fmt.Sprintf("%s (%s TODO)", plural(caseCount, "case"), plural(len(cases), "expected value")), true
}

func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func resultVar(resultType string) string {
	if resultType == "error" {
		return "err"
	}
	return "ok"
}

// An empty case Value leaves the input at its zero value.
func (b *builder) validationInput(param detect.Field, exportedFunc bool) (inputType string, cases []edgeCase, ok bool) {
	switch {
	case param.Type == "time.Time":
		b.use("time")
		for _, unix := range unixCases {
			cases = append(cases, edgeCase{Name: unix.Name, Value: "time.Unix(" + unix.Value + ", 0).UTC()"})
		}
		return param.Type, cases, true
	case strings.HasPrefix(param.Type, "[]"):
		return param.Type, b.sliceCases(param.Type), true
	case strings.HasPrefix(param.Type, "..."):
		return "", nil, false
	case param.Type != "":
		return param.Type, b.edgeCases(param.Type), true
	case param.Underlying != "":
		// The edge values of the basic type, converted: Slug(""), Slug("😀").
		var cases []edgeCase
		for _, edge := range b.edgeCases(param.Underlying) {
			cases = append(cases, edgeCase{Name: edge.Name, Value: param.Expr + "(" + edge.Value + ")"})
		}
		return param.Expr, cases, true
	case localTypeExpr.MatchString(param.Expr):
		return param.Expr, localTypeCases(param.Expr, exportedFunc), true
	default:
		return "", nil, false
	}
}

func (b *builder) sliceCases(sliceType string) []edgeCase {
	cases := []edgeCase{
		{Name: "nil"},
		{Name: "empty", Value: sliceType + "{}"},
	}
	sample := b.sample(strings.TrimPrefix(sliceType, "[]"))
	if sample != "" {
		cases = append(cases, edgeCase{Name: "one element", Value: sliceType + "{" + sample + "}"})
	}
	return cases
}

// nil is only a case for exported funcs: a caller outside the package may pass it, while an
// unexported helper is called by code that already checked.
func localTypeCases(typeExpr string, exported bool) []edgeCase {
	typeName, isPointer := strings.CutPrefix(typeExpr, "*")
	switch {
	case isPointer && exported:
		return []edgeCase{{Name: "nil"}, {Name: "zero value", Value: "new(" + typeName + ")"}}
	case isPointer:
		return []edgeCase{{Name: "zero value", Value: "new(" + typeName + ")"}}
	default:
		return []edgeCase{{Name: "zero value"}}
	}
}

// funcPart is Validate for User.Validate.
func funcPart(name string) string {
	_, method, isMethod := strings.Cut(name, ".")
	if isMethod {
		return method
	}
	return name
}

func exported(name string) bool {
	return name != "" && unicode.IsUpper([]rune(name)[0])
}
