package fixture

import (
	"net/http"
	"strconv"
)

// The false positives the real-world corpus turned up (docs/real-world.md), as near misses.

// Middleware returns a closure: two calls are never equal, so determinism can't be checked.
func Middleware(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

type PositionalArgs func(args []string) error

// MinimumNArgs returns a func type of the package (cobra).
func MinimumNArgs(n int) PositionalArgs {
	return func(args []string) error { return nil }
}

// Handler returns an interface holding a func.
func Handler(name string) http.Handler { return http.NotFoundHandler() }

func Numbers(n int) <-chan int { return make(chan int, n) }

// Panics on bad input by convention (shopspring/decimal: RequireFromString).
func MustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func RequireID(s string) string { return s }

// Half panics if n is odd: its doc says so, so a panic is no finding.
func Half(n int) int {
	if n%2 != 0 {
		panic("odd")
	}
	return n / 2
}

// Still a candidate: error is an interface, but one that compares fine.
func Atoi(s string) (int, error) { return strconv.Atoi(s) }

// TODO: panics on an empty s. Still a candidate: a known bug isn't a contract.
func First(s string) byte { return s[0] }

// Still a candidate: Must is only a prefix at a word boundary.
func Mustard(n int) int { return n }

type Handler func()

// A named type over a func type is a func type too.
type Middleware Handler

func Chain(n int) Middleware { return nil }

// Halve panics in debug builds if n is odd: a documented panic, whatever the build.
func Halve(n int) int { return n / 2 }
