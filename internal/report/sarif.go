package report

import (
	"encoding/json"
	"io"
	"os"
	"slices"
)

// SARIF 2.1.0, the format GitHub code scanning and most CI dashboards read. Unlike annotations
// it has no limit, and code scanning tracks each issue across runs.
type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string        `json:"id"`
	ShortDescription sarifText     `json:"shortDescription"`
	HelpURI          string        `json:"helpUri"`
	Default          sarifSettings `json:"defaultConfiguration"`
}

type sarifSettings struct {
	Level string `json:"level"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	Physical sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	Artifact sarifArtifact `json:"artifactLocation"`
	Region   *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

const informationURI = "https://github.com/Mirac61/yawntest"

var ruleDescriptions = map[string]string{
	"untested":   "Code yawntest can write a test for, and no test covers",
	"hint":       "Code that is likely wrong in a way tests rarely catch",
	"error-path": "An error return no test reaches",
	"finding":    "A generated test that fails on the current code",
}

var sarifLevels = map[string]string{"notice": "note", "warning": "warning", "error": "error"}

// SARIF writes the result as one SARIF run. Issues without a file, like a package whose
// tests don't build, have nothing to point at and are left out; the text report has them.
func SARIF(w io.Writer, result Result, version string) error {
	workspace := os.Getenv("GITHUB_WORKSPACE")
	run := sarifRun{
		Tool:    sarifTool{Driver: sarifDriver{Name: "yawntest", Version: version, InformationURI: informationURI}},
		Results: []sarifResult{},
	}

	rules := map[string]string{} // id → level
	for _, issue := range issues(result) {
		if issue.file == "" {
			continue
		}
		level := sarifLevels[issue.level]
		rules[issue.rule] = level
		physical := sarifPhysical{Artifact: sarifArtifact{URI: repoPath(workspace, issue.file), URIBaseID: "%SRCROOT%"}}
		if issue.line > 0 {
			physical.Region = &sarifRegion{StartLine: issue.line}
		}
		run.Results = append(run.Results, sarifResult{
			RuleID:    issue.rule,
			Level:     level,
			Message:   sarifText{Text: issue.message},
			Locations: []sarifLocation{{Physical: physical}},
		})
	}

	ids := make([]string, 0, len(rules))
	for id := range rules {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	run.Tool.Driver.Rules = []sarifRule{}
	for _, id := range ids {
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{
			ID:               id,
			ShortDescription: sarifText{Text: ruleDescriptions[ruleKind(id)]},
			HelpURI:          informationURI + "#readme",
			Default:          sarifSettings{Level: rules[id]},
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs:    []sarifRun{run},
	})
}
