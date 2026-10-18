package calc

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"
)

func Add(a, b int) int { return a + b }

func Join(parts []string, sep string) string { return strings.Join(parts, sep) }

func Sum(xs ...float64) (s float64) {
	for _, x := range xs {
		s += x
	}
	return s
}

func Days(start, end time.Time) int { return int(end.Sub(start).Hours() / 24) }

func Label(n int) string { return fmt.Sprintf("#%d", n) }

// impure or out of scope below

func ReadName(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func Roll(n int) int { return rand.IntN(n) }

func Age(born time.Time) int { return time.Now().Year() - born.Year() }

// Writing package state: calling twice isn't the same as calling once, and a fuzz test
// calling it would change what the package's other tests see.

var (
	nodeID   [6]byte
	calls    int
	lastName string
	cache    = map[string]int{}
)

func SetNode(id []byte) bool {
	if len(id) < 6 {
		return false
	}
	copy(nodeID[:], id)
	return true
}

func Counted(n int) int {
	calls++
	return n
}

func Remember(s string) string {
	lastName = s
	return s
}

func Memo(s string) int {
	cache[s] = len(s)
	return cache[s]
}

func Shout(s string) string {
	fmt.Println(s)
	return s
}

// Locals and params with the names of package vars are no package state.

func ShadowParam(calls int) int {
	calls *= 2
	return calls
}

func ShadowLocal(s string) string {
	lastName := s
	lastName += "!"
	return lastName
}

func Count(m map[string]int) int { return len(m) }

func NoResult(n int) {}

func NoParams() int { return 1 }

func add(a, b int) int { return a + b }

func Max[T int | float64](a, b T) T { return max(a, b) }
