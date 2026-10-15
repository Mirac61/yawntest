package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

const usage = `usage: yawntest <command> [flags] [path]

commands:
  check  list untested candidates, hints and error paths no test reaches; writes nothing
  gen    write tests for the untested candidates
  run    gen, then run the generated tests and report failures as findings

path defaults to ".", "./..." works too. See yawntest <command> -h for the flags.
yawntest --version prints the version.
`

var helpArgs = []string{"help", "-h", "-help", "--help"}

var formats = []string{"text", "json", "github", "sarif"}

// errUsage is never printed: parseArgs already told the user what's wrong.
var errUsage = errors.New("usage")

// parseArgs returns flag.ErrHelp after printing help, and errUsage after printing a mistake.
func parseArgs(args []string, output io.Writer) (options, error) {
	if len(args) == 0 {
		fmt.Fprint(output, usage)
		return options{}, errUsage
	}
	if slices.Contains(helpArgs, args[0]) {
		fmt.Fprint(output, usage)
		return options{}, flag.ErrHelp
	}

	opts := options{command: args[0], root: ".", format: "text"}
	flags, ok := commandFlags(&opts)
	if !ok {
		fmt.Fprintf(output, "yawntest: unknown command %q\n\n%s", opts.command, usage)
		return options{}, errUsage
	}
	flags.SetOutput(output)

	// The flag package stops at the first non-flag, so flags after the path are parsed in a second round.
	var paths []string
	rest := args[1:]
	for {
		if err := flags.Parse(rest); err != nil {
			return options{}, err
		}
		if flags.NArg() == 0 {
			break
		}
		paths = append(paths, flags.Arg(0))
		rest = flags.Args()[1:]
	}
	switch len(paths) {
	case 0:
	case 1:
		opts.root = strings.TrimSuffix(paths[0], "/...")
		if opts.root == "" || opts.root == "..." {
			opts.root = "."
		}
	default:
		fmt.Fprintf(output, "yawntest: one path at most, got %q\n", paths)
		return options{}, errUsage
	}

	if opts.json {
		opts.format = "json"
	}
	if !slices.Contains(formats, opts.format) {
		fmt.Fprintf(output, "yawntest: --format must be one of %s, got %q\n", strings.Join(formats, ", "), opts.format)
		return options{}, errUsage
	}
	return opts, nil
}

// commandFlags binds the flags of opts.command to opts, or returns false if there's no such command.
func commandFlags(opts *options) (*flag.FlagSet, bool) {
	flags := flag.NewFlagSet("yawntest "+opts.command, flag.ContinueOnError)
	switch opts.command {
	case "check":
		flags.BoolVar(&opts.static, "static", false, "don't run tests: no error paths, and only names count as tested")
	case "gen":
		flags.BoolVar(&opts.force, "force", false, "overwrite existing yawntest files")
		flags.BoolVar(&opts.all, "all", false, "also generate tests for code that already has tests")
		flags.BoolVar(&opts.static, "static", false, "don't run tests: only names count as tested")
	case "run":
		flags.BoolVar(&opts.force, "force", false, "overwrite existing yawntest files")
		flags.BoolVar(&opts.all, "all", false, "also generate tests for code that already has tests")
		flags.DurationVar(&opts.fuzztime, "fuzztime", 0, "after the seeds, fuzz each generated fuzz test for this `duration`, e.g. 10s")
	default:
		return nil, false
	}
	flags.BoolVar(&opts.changed, "changed", false, "only look at code changed since the last commit")
	flags.StringVar(&opts.since, "since", "", "only look at code changed since the merge base with `ref`, e.g. origin/main")
	flags.StringVar(&opts.format, "format", "text", "output `format`: text, json, github for annotations in GitHub Actions, or sarif for code scanning")
	flags.StringVar(&opts.sarifFile, "sarif", "", "also write the result as SARIF to `file`, whatever --format is")
	flags.BoolVar(&opts.json, "json", false, "short for --format json")
	flags.StringVar(&opts.config, "config", "", "use this `file` instead of the .yawntest.json found in the path or above it")

	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: yawntest %s [flags] [path]\n", opts.command)
		flags.PrintDefaults()
	}
	return flags, true
}
