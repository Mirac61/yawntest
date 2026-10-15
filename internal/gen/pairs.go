package gen

import (
	"fmt"
	"text/template"

	"github.com/Mirac61/yawntest/internal/detect"
)

var pairRoundtripTemplate = template.Must(template.New("pairRoundtrip").Parse(`
// yawntest: roundtrip / {{.Decode}}({{.Encode}}(x)) gives x back
func FuzzYawntest_{{.Encode}}_Roundtrip(f *testing.F) {
{{- range .Seeds}}
	f.Add({{.}})
{{- end}}

	f.Fuzz(func(t *testing.T, {{.FuzzArg}} {{.ArgType}}) {
		{{.NoPanic}}
{{- if .Setup}}
		{{.Setup}}
{{- end}}
{{- if .EncodeErr}}
		encoded, err := {{.Encode}}({{.Arg}})
		if err != nil {
			return // inputs {{.Encode}} refuses have nothing to round-trip
		}
{{- else}}
		encoded := {{.Encode}}({{.Arg}})
{{- end}}
{{- if .DecodeErr}}
		decoded, err := {{.Decode}}(encoded)
		if err != nil {
			t.Fatalf("{{.Decode}}({{.Encode}}(%#v)) failed: %v", {{.Arg}}, err)
		}
{{- else}}
		decoded := {{.Decode}}(encoded)
{{- end}}
{{- if .Bytes}}
		if !bytes.Equal(decoded, {{.Arg}}) {
{{- else}}
		if decoded != {{.Arg}} {
{{- end}}
			t.Errorf("{{.Decode}}({{.Encode}}(%#v)) = %#v, want the input back (encoded: %#v)", {{.Arg}}, decoded, encoded)
		}
	})
}
`))

var enumRoundtripTemplate = template.Must(template.New("enumRoundtrip").Parse(`
// yawntest: enum-roundtrip / {{.Parse}}(c.String()) gives c back for every {{.Type}}
func TestYawntest_{{.Parse}}_EnumRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		want {{.Type}}
	}{
{{- range .Consts}}
		{name: {{printf "%q" .}}, want: {{.}}},
{{- end}}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			{{.NoPanic}}
			text := test.want.String()
			got, err := {{.Parse}}(text)
			if err != nil {
				t.Fatalf("{{.Parse}}(%q) failed: %v, want %s", text, err, test.name)
			}
			if got != test.want {
				t.Errorf("{{.Parse}}(%q) = %v, want %s", text, got, test.name)
			}
		})
	}
}
`))

func (b *builder) pairRoundtrip(match detect.Match) string {
	b.use("testing")
	arg := b.fuzzArg(0, match.Params[0])
	bytesInput := match.Params[0].Type == "[]byte"
	if bytesInput {
		b.use("bytes")
	}

	b.execute(pairRoundtripTemplate, map[string]any{
		"Encode":    match.Name,
		"Decode":    match.Partner,
		"Seeds":     arg.seeds,
		"FuzzArg":   arg.name,
		"Setup":     arg.setup,
		"Arg":       arg.callArg,
		"ArgType":   arg.fuzzType,
		"NoPanic":   noPanic(""),
		"EncodeErr": len(match.Results) == 2,
		"DecodeErr": len(match.PartnerResults) == 2,
		"Bytes":     bytesInput,
	})
	return fmt.Sprintf("fuzz, %d seeds, %s back", len(arg.seeds), match.Partner)
}

func (b *builder) enumRoundtrip(match detect.Match) string {
	b.use("testing")
	b.execute(enumRoundtripTemplate, map[string]any{
		"Parse":   match.Name,
		"Type":    match.Partner,
		"Consts":  match.Consts,
		"NoPanic": noPanic(""),
	})
	return plural(len(match.Consts), "const")
}
