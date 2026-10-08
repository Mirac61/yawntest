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

	opts := options{command: args[0], root: "."}
	flags, ok := commandFlags(&opts)
	if !ok {
		fmt.Fprintf(output, "yawntest: unknown command %q\n\n%s", opts.command, usage)
		return options{}, errUsage
	}
	flags.SetOutput(output)
	if err := flags.Parse(args[1:]); err != nil {
		return options{}, err
	}

	switch flags.NArg() {
	case 0:
	case 1:
		opts.root = strings.TrimSuffix(flags.Arg(0), "/...")
	default:
		// Parsing stops at the path, so a flag after it would be silently ignored.
		fmt.Fprintf(output, "yawntest: flags go before the path, got %q\n", flags.Args())
		return options{}, errUsage
	}
	return opts, nil
}

// commandFlags binds the flags of opts.command to opts, or returns false if there's no such command.
func commandFlags(opts *options) (*flag.FlagSet, bool) {
	flags := flag.NewFlagSet("yawntest "+opts.command, flag.ContinueOnError)
	switch opts.command {
	case "check":
	case "gen", "run":
		flags.BoolVar(&opts.force, "force", false, "overwrite existing yawntest files")
	default:
		return nil, false
	}
	flags.BoolVar(&opts.changed, "changed", false, "only look at code changed since the last commit")
	flags.BoolVar(&opts.json, "json", false, "print the result as JSON")

	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: yawntest %s [flags] [path]\n", opts.command)
		flags.PrintDefaults()
	}
	return flags, true
}
