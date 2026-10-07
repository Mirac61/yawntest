package gen

import (
	"strings"
	"text/template"

	"github.com/Mirac61/lazytest/internal/detect"
)

var httpTemplate = template.Must(template.New("http").Parse(`
// lazytest: http-handler / bad input never gives 5xx{{if .DecodesJSON}}, invalid JSON gives 4xx{{end}}
func TestLazytest_{{.TestName}}_BadInput(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		body    string
		want4xx bool
	}{
		{name: "empty request", method: http.MethodGet},
{{- if .DecodesJSON}}
		{name: "invalid json", method: http.MethodPost, body: "{", want4xx: true},
		{name: "empty json body", method: http.MethodPost, want4xx: true},
{{- end}}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
{{- if .IsMethod}}
			// The receiver is zero-valued; give it real dependencies if the handler needs them.
{{- end}}
			handler := {{.Handler}}
			request := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()

			handler(recorder, request)

			status := recorder.Code
			if status >= 500 {
				t.Errorf("status = %d, want < 500 for bad input", status)
			}
			if test.want4xx && status < 400 {
				t.Errorf("status = %d, want 4xx", status)
			}
		})
	}
}
`))

// Constructors with params are skipped: lazytest can't invent their dependencies.
func (b *builder) httpHandler(match detect.Match) (summary string, ok bool) {
	handler := callable(match.Name)
	isConstructor := len(match.Results) == 1
	if isConstructor {
		if len(match.Params) > 0 {
			return "", false
		}
		handler += "()"
	}

	b.use("net/http", "net/http/httptest", "strings", "testing")
	b.execute(httpTemplate, map[string]any{
		"TestName":    testName(match.Name),
		"Handler":     handler,
		"IsMethod":    strings.Contains(match.Name, "."),
		"DecodesJSON": match.DecodesJSON,
	})

	if match.DecodesJSON {
		return "3 cases", true
	}
	return "1 case", true
}
