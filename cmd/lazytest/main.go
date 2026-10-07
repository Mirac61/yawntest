package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Mirac61/lazytest/internal/gen"
	"github.com/Mirac61/lazytest/internal/report"
	"github.com/Mirac61/lazytest/internal/scan"
)

const (
	exitMissingTests = 1 // lets --check fail a CI job
	exitError        = 2
)

func main() {
	check := flag.Bool("check", false, "report untested patterns without writing files")
	force := flag.Bool("force", false, "regenerate existing lazytest files")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: lazytest [--check] [--force] [path]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *check {
		runCheck(rootArg())
		return
	}
	runGenerate(rootArg(), *force)
}

func runCheck(root string) {
	files, err := scan.Untested(root, true)
	if err != nil {
		fail(err)
	}
	if err := report.Untested(os.Stdout, files); err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
	if len(files) > 0 {
		os.Exit(exitMissingTests)
	}
}

// Without --force existing lazytest files count as tests, so only sources with new candidates show up.
func runGenerate(root string, force bool) {
	files, err := scan.Untested(root, !force)
	if err != nil {
		fail(err)
	}

	var results []report.GeneratedFile
	for _, file := range files {
		result, err := generateFile(file, force)
		if err != nil {
			fail(err)
		}
		results = append(results, result)
	}

	if err := report.Generated(os.Stdout, results); err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
}

func generateFile(file scan.File, force bool) (report.GeneratedFile, error) {
	result := report.GeneratedFile{Source: file.Path, TestPath: gen.TestPath(file.Path)}

	output, err := gen.File(file.Package, file.Matches)
	if err != nil {
		return result, fmt.Errorf("generate %s: %w", file.Path, err)
	}
	result.Output = output
	if output.Code == nil {
		return result, nil
	}

	exists, err := fileExists(result.TestPath)
	if err != nil {
		return result, err
	}
	if exists && !force {
		return result, nil
	}

	if err := os.WriteFile(result.TestPath, output.Code, 0o644); err != nil {
		return result, fmt.Errorf("write %s: %w", result.TestPath, err)
	}
	result.Written = true
	return result, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazytest:", err)
	os.Exit(exitError)
}

func rootArg() string {
	if flag.NArg() == 0 {
		return "."
	}
	return strings.TrimSuffix(flag.Arg(0), "/...")
}
