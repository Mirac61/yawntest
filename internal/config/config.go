// Package config reads .yawntest.json, the per-repo settings: paths to leave out and checks to
// turn off. JSON, because the standard library has no YAML parser.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Mirac61/yawntest/internal/detect"
	"github.com/Mirac61/yawntest/internal/scan"
)

const FileName = ".yawntest.json"

// Config is the parsed file. The zero value excludes and disables nothing.
type Config struct {
	Exclude []string `json:"exclude"` // globs relative to the file, ** for any number of dirs
	Disable []string `json:"disable"` // see Disableable

	dir string // where the file is; Exclude is relative to it
}

// Disableable lists what Disable may name.
func Disableable() []string {
	keys := []string{"error-paths"}
	for _, pattern := range []detect.Pattern{
		detect.PatternHTTPHandler, detect.PatternJSONRoundtrip, detect.PatternPureFunc,
		detect.PatternValidation, detect.PatternRoundtrip, detect.PatternEnumRoundtrip,
	} {
		keys = append(keys, "pattern:"+string(pattern))
	}
	return append(keys, "hint:money", "hint:dates", "hint:auth")
}

// Load reads the file at explicit, or else finds .yawntest.json in root or a dir above it, up
// to the module or repo root. No file is no error: Load returns the zero Config.
func Load(explicit, root string) (Config, error) {
	file := explicit
	if file == "" {
		var err error
		if file, err = find(root); err != nil || file == "" {
			return Config{}, err
		}
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return Config{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("%s: %w", file, err)
	}
	for _, key := range config.Disable {
		if !slices.Contains(Disableable(), key) {
			return Config{}, fmt.Errorf("%s: can't disable %q, choose from %s", file, key, strings.Join(Disableable(), ", "))
		}
	}
	for _, pattern := range config.Exclude {
		if _, err := path.Match(strings.ReplaceAll(pattern, "**", "x"), ""); err != nil {
			return Config{}, fmt.Errorf("%s: bad exclude pattern %q: %w", file, pattern, err)
		}
	}
	if config.dir, err = filepath.Abs(filepath.Dir(file)); err != nil {
		return Config{}, err
	}
	return config, nil
}

// find stops at the first dir holding go.mod or .git: a config above the module is someone else's.
func find(root string) (string, error) {
	dir, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		candidate := filepath.Join(dir, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if exists(filepath.Join(dir, "go.mod")) || exists(filepath.Join(dir, ".git")) {
			return "", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (c Config) Disabled(key string) bool {
	return slices.Contains(c.Disable, key)
}

// Excluded reports whether a path, as the scan walks it, matches an Exclude pattern. A pattern
// matching a dir excludes everything below it.
func (c Config) Excluded(walked string) bool {
	if len(c.Exclude) == 0 {
		return false
	}
	absolute, err := filepath.Abs(walked)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(c.dir, absolute)
	if err != nil || strings.HasPrefix(relative, "..") {
		return false
	}
	relative = filepath.ToSlash(relative)
	for _, pattern := range c.Exclude {
		if matchesOrParent(strings.TrimSuffix(pattern, "/"), relative) {
			return true
		}
	}
	return false
}

func matchesOrParent(pattern, relative string) bool {
	segments := strings.Split(relative, "/")
	for i := len(segments); i > 0; i-- {
		if match(strings.Split(pattern, "/"), segments[:i]) {
			return true
		}
	}
	return false
}

// match is path.Match per segment, plus ** for zero or more segments.
func match(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return len(segments) == 0
	}
	if pattern[0] == "**" {
		for skip := 0; skip <= len(segments); skip++ {
			if match(pattern[1:], segments[skip:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], segments[0])
	return err == nil && ok && match(pattern[1:], segments[1:])
}

// Apply drops the disabled patterns and hints from scanned files.
func (c Config) Apply(files []scan.File) []scan.File {
	if len(c.Disable) == 0 {
		return files
	}
	var kept []scan.File
	for _, file := range files {
		var matches []detect.Match
		for _, match := range file.Matches {
			if !c.Disabled("pattern:" + string(match.Pattern)) {
				matches = append(matches, match)
			}
		}
		var hints []detect.Hint
		for _, hint := range file.Hints {
			if !c.Disabled("hint:" + hint.Domain) {
				hints = append(hints, hint)
			}
		}
		file.Matches, file.Hints = matches, hints
		if c.Disabled("error-paths") {
			file.ErrorReturns = nil
		}
		kept = append(kept, file)
	}
	return kept
}
