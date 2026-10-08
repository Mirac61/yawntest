package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/Mirac61/yawntest/internal/changes"
	"github.com/Mirac61/yawntest/internal/gen"
	"github.com/Mirac61/yawntest/internal/report"
	"github.com/Mirac61/yawntest/internal/run"
	"github.com/Mirac61/yawntest/internal/scan"
)

const (
	exitProblems = 1 // see report.Result.HasProblems; lets CI fail
	exitError    = 2
)

type options struct {
	command string // check, gen or run
	root    string
	force   bool
	changed bool
	json    bool
}

const usage = `usage: yawntest <command> [flags] [path]

commands:
  check  list untested candidates, hints and error paths no test reaches; writes nothing
  gen    write tests for the untested candidates
  run    gen, then run the generated tests and report failures as findings

path defaults to ".", "./..." works too. See yawntest <command> -h for the flags.
`

func main() {
	opts, err := parseArgs(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		os.Exit(exitError)
	}

	result := collect(opts)
	if opts.json {
		mustReport(report.JSON(os.Stdout, result))
	} else {
		printText(opts, result)
	}
	if result.HasProblems() {
		os.Exit(exitProblems)
	}
}

// parseArgs prints usage and errors to output itself; an error only tells main to stop.
func parseArgs(args []string, output io.Writer) (options, error) {
	if len(args) == 0 {
		fmt.Fprint(output, usage)
		return options{}, errors.New("no command")
	}

	opts := options{command: args[0], root: "."}
	flags := flag.NewFlagSet("yawntest "+opts.command, flag.ContinueOnError)
	flags.SetOutput(output)
	switch opts.command {
	case "check":
	case "gen", "run":
		flags.BoolVar(&opts.force, "force", false, "overwrite existing yawntest files")
	case "help", "-h", "-help", "--help":
		fmt.Fprint(output, usage)
		return options{}, flag.ErrHelp
	default:
		fmt.Fprintf(output, "yawntest: unknown command %q\n\n%s", opts.command, usage)
		return options{}, errors.New("unknown command")
	}
	flags.BoolVar(&opts.changed, "changed", false, "only look at code changed since the last commit")
	flags.BoolVar(&opts.json, "json", false, "print the result as JSON")
	flags.Usage = func() {
		fmt.Fprintf(output, "usage: yawntest %s [flags] [path]\n", opts.command)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args[1:]); err != nil {
		return options{}, err
	}
	// Go's flag parsing stops at the path, so a flag after it would be silently ignored.
	if flags.NArg() > 1 {
		fmt.Fprintf(output, "yawntest: one path only, flags go before it; got %q\n", flags.Args())
		return options{}, errors.New("too many args")
	}
	if flags.NArg() == 1 {
		opts.root = strings.TrimSuffix(flags.Arg(0), "/...")
	}
	return opts, nil
}

func collect(opts options) report.Result {
	if opts.command == "check" {
		return check(opts)
	}

	result := report.Result{Generated: generate(opts)}
	if opts.command == "run" {
		dirs := must(scan.Dirs(opts.root, scan.IsYawntestFile))
		result.Findings = must(run.YawntestTests(dirs))
	}
	return result
}

func printText(opts options, result report.Result) {
	if opts.command == "check" {
		mustReport(report.Untested(os.Stdout, result.Untested))
		mustReport(report.Hints(os.Stdout, result.Untested))
		mustReport(report.ErrorPaths(os.Stdout, result.ErrorPaths))
		return
	}

	mustReport(report.Generated(os.Stdout, result.Generated))
	if opts.command == "run" {
		mustReport(report.Findings(os.Stdout, result.Findings))
	}
}

func check(opts options) report.Result {
	files := must(scan.Untested(opts.root, true))
	sourceDirs := must(scan.Dirs(opts.root, scan.IsSourceFile))
	errorPaths := must(run.UntestedErrorPaths(sourceDirs))
	if opts.changed {
		changed := must(changes.SinceHEAD(opts.root))
		files = must(changes.KeepChangedMatches(files, changed))
		errorPaths = must(changes.KeepChangedErrorPaths(errorPaths, changed))
	}
	files = must(run.WithoutCovered(files))
	return report.Result{Untested: files, ErrorPaths: errorPaths}
}

// Without --force existing yawntest files count as tests, so only sources with new candidates show up.
func generate(opts options) []report.GeneratedFile {
	files := must(scan.Untested(opts.root, !opts.force))
	if opts.changed {
		changed := must(changes.SinceHEAD(opts.root))
		files = must(changes.KeepChangedFiles(files, changed))
	}
	files = must(run.WithoutCovered(files))

	var results []report.GeneratedFile
	for _, file := range files {
		if len(file.Matches) > 0 {
			results = append(results, must(generateFile(file, opts.force)))
		}
	}
	return results
}

func generateFile(file scan.File, force bool) (report.GeneratedFile, error) {
	result := report.GeneratedFile{Source: file.Path, TestPath: gen.TestPath(file.Path)}

	output, err := gen.File(file.Package, file.BuildConstraint, file.Matches)
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

// Every error in a CLI run is fatal, so must ends the program instead of returning.
func must[T any](value T, err error) T {
	if err != nil {
		fail(err)
	}
	return value
}

func mustReport(err error) {
	if err != nil {
		fail(fmt.Errorf("write report: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "yawntest:", err)
	os.Exit(exitError)
}
