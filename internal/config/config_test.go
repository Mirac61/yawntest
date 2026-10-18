package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/scan"
)

func TestExcluded(t *testing.T) {
	config := Config{Exclude: []string{"internal/legacy", "**/*_mock.go", "gen/*.go", "docs/**/x.go"}, dir: "/repo"}
	tests := []struct {
		path string
		want bool
	}{
		{"/repo/internal/legacy", true},
		{"/repo/internal/legacy/old/a.go", true}, // below an excluded dir
		{"/repo/internal/legacyish/a.go", false},
		{"/repo/db_mock.go", true}, // ** matches no dirs too
		{"/repo/a/b/c/db_mock.go", true},
		{"/repo/gen/a.go", true},
		{"/repo/gen/sub/a.go", false}, // * stays within one dir
		{"/repo/docs/x.go", true},
		{"/repo/docs/a/b/x.go", true},
		{"/elsewhere/internal/legacy/a.go", false},
	}
	for _, test := range tests {
		if got := config.Excluded(test.path); got != test.want {
			t.Errorf("Excluded(%s) = %v, want %v", test.path, got, test.want)
		}
	}
	if (Config{}).Excluded("/repo/a.go") {
		t.Error("the zero Config excludes something")
	}
}

func TestLoad(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, "go.mod"), "module repo\n")
	write(t, filepath.Join(repo, FileName), `{"exclude": ["legacy"], "disable": ["hint:money", "error-paths"]}`)
	sub := filepath.Join(repo, "internal", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Found from a dir below it, and Exclude stays relative to the file.
	config, err := Load("", sub)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !config.Disabled("hint:money") || !config.Disabled("error-paths") || config.Disabled("hint:auth") {
		t.Errorf("Disable = %v", config.Disable)
	}
	if !config.Excluded(filepath.Join(repo, "legacy", "a.go")) {
		t.Error("legacy/a.go not excluded")
	}

	// The search stops at the module root: a file above it belongs to someone else.
	outer := t.TempDir()
	write(t, filepath.Join(outer, FileName), `{"disable": ["hint:money"]}`)
	module := filepath.Join(outer, "module")
	write(t, filepath.Join(module, "go.mod"), "module m\n")
	if config, err := Load("", module); err != nil || len(config.Disable) > 0 {
		t.Errorf("Load above the module = %+v, %v; want nothing", config, err)
	}
}

func TestLoadRejectsMistakes(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct{ content, wantErr string }{
		{`{"disable": ["hint:mony"]}`, `can't disable "hint:mony", choose from error-paths, pattern:http-handler`},
		{`{"exlude": ["x"]}`, `unknown field "exlude"`},
		{`{"exclude": ["[x"]}`, `bad exclude pattern "[x"`},
		{`{"exclude": `, `unexpected EOF`},
	} {
		file := filepath.Join(dir, "config.json")
		write(t, file, test.content)
		if _, err := Load(file, dir); err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Errorf("Load(%s) err = %v, want %q", test.content, err, test.wantErr)
		}
	}
	if _, err := Load(filepath.Join(dir, "missing.json"), dir); err == nil {
		t.Error("a missing --config file must be an error")
	}
}

func TestApply(t *testing.T) {
	files := []scan.File{{
		Path: "a.go",
		Matches: []detect.Match{
			{Name: "A", Pattern: detect.PatternPureFunc},
			{Name: "B", Pattern: detect.PatternValidation},
		},
		Hints:        []detect.Hint{{Domain: "money"}, {Domain: "auth"}},
		ErrorReturns: []detect.ErrorReturn{{Line: 3}},
	}}
	config := Config{Disable: []string{"pattern:validation", "hint:money", "error-paths"}}
	got := config.Apply(files)[0]
	if len(got.Matches) != 1 || got.Matches[0].Name != "A" || len(got.Hints) != 1 || got.Hints[0].Domain != "auth" || got.ErrorReturns != nil {
		t.Errorf("Apply = %+v", got)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
