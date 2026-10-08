package gen

import (
	"text/template"

	"github.com/Mirac61/yawntest/internal/detect"
)

var roundtripTemplate = template.Must(template.New("roundtrip").Parse(`
// lazytest: json-roundtrip
func TestLazytest_{{.TestName}}_JSONRoundtrip(t *testing.T) {
	want := {{.Type}}{
	{{- range .Fields}}
		{{.Name}}: {{.Value}},
	{{- end}}
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got {{.Type}}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roundtrip changed the value:\n got %+v\nwant %+v", got, want)
	}
}
`))

type fieldValue struct {
	Name  string
	Value string
}

// Fields without a sample value stay zero, which round-trips anyway.
func (b *builder) roundtrip(match detect.Match) string {
	b.use("encoding/json", "reflect", "testing")

	var fields []fieldValue
	for _, field := range match.Fields {
		value := b.sample(field.Type)
		if value != "" {
			fields = append(fields, fieldValue{Name: field.Name, Value: value})
		}
	}

	b.execute(roundtripTemplate, map[string]any{
		"Type":     match.Name,
		"TestName": testName(match.Name),
		"Fields":   fields,
	})
	return "1 test"
}
