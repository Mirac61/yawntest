package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Mirac61/lazytest/internal/report"
	"github.com/Mirac61/lazytest/internal/scan"
)

const (
	exitMissingTests = 1 // lets --check fail a CI job
	exitError        = 2
)

func main() {
	check := flag.Bool("check", false, "report untested patterns without writing files")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: lazytest [--check] [path]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if !*check {
		fmt.Fprintln(os.Stderr, "lazytest: generation is not implemented yet, use --check")
		os.Exit(exitError)
	}

	matches, err := scan.Untested(rootArg())
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazytest:", err)
		os.Exit(exitError)
	}
	if err := report.Text(os.Stdout, matches); err != nil {
		fmt.Fprintln(os.Stderr, "lazytest: write report:", err)
		os.Exit(exitError)
	}
	if len(matches) > 0 {
		os.Exit(exitMissingTests)
	}
}

func rootArg() string {
	if flag.NArg() == 0 {
		return "."
	}
	return strings.TrimSuffix(flag.Arg(0), "/...")
}
