package report

import (
	"encoding/json"
	"io"
)

type jsonResult struct {
	Candidates []jsonCandidate     `json:"candidates,omitempty"`
	Hints      []jsonHint          `json:"hints,omitempty"`
	ErrorPaths []jsonErrorPath     `json:"errorPaths,omitempty"`
	Generated  []jsonGeneratedFile `json:"generated,omitempty"`
	Findings   []jsonFinding       `json:"findings,omitempty"`
}

type jsonCandidate struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

type jsonHint struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Domain  string `json:"domain"`
	Message string `json:"message"`
}

type jsonErrorPath struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Code string `json:"code"`
}

type jsonGeneratedFile struct {
	Source   string     `json:"source"`
	TestFile string     `json:"testFile"`
	Written  bool       `json:"written"`
	Tests    []jsonTest `json:"tests"`
	Skipped  []string   `json:"skipped,omitempty"`
}

type jsonTest struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Summary string `json:"summary"`
}

type jsonFinding struct {
	Dir        string `json:"dir"`
	Test       string `json:"test,omitempty"`
	Message    string `json:"message"`
	NeedsSetup bool   `json:"needsSetup,omitempty"`
}

func JSON(w io.Writer, result Result) error {
	var out jsonResult
	for _, file := range result.Untested {
		for _, match := range file.Matches {
			out.Candidates = append(out.Candidates, jsonCandidate{
				File: file.Path, Line: match.Line, Name: match.Name, Pattern: string(match.Pattern), Reason: match.Reason,
			})
		}
		for _, hint := range file.Hints {
			out.Hints = append(out.Hints, jsonHint{File: file.Path, Line: hint.Line, Domain: hint.Domain, Message: hint.Message})
		}
	}
	for _, path := range result.ErrorPaths {
		out.ErrorPaths = append(out.ErrorPaths, jsonErrorPath{File: path.File, Line: path.Line, Code: path.Code})
	}
	for _, file := range result.Generated {
		out.Generated = append(out.Generated, generatedJSON(file))
	}
	for _, finding := range result.Findings {
		out.Findings = append(out.Findings, jsonFinding{
			Dir: finding.Dir, Test: finding.Test, Message: finding.Message, NeedsSetup: finding.NeedsSetup,
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func generatedJSON(file GeneratedFile) jsonGeneratedFile {
	generated := jsonGeneratedFile{Source: file.Source, TestFile: file.TestPath, Written: file.Written}
	for _, test := range file.Output.Tests {
		generated.Tests = append(generated.Tests, jsonTest{
			Name: test.Match.Name, Pattern: string(test.Match.Pattern), Summary: test.Summary,
		})
	}
	for _, match := range file.Output.Skipped {
		generated.Skipped = append(generated.Skipped, match.Name)
	}
	return generated
}
