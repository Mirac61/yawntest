package fixture

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func Add(a, b int) int { return a + b }

// Returns NaN for NaN input, which must still count as deterministic.
func Clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(x, hi)) }

func Join(parts []string, sep string) string { return strings.Join(parts, sep) }

func Sum(xs ...float64) float64 {
	total := 0.0
	for _, x := range xs {
		total += x
	}
	return total
}

func Days(start, end time.Time) int { return int(end.Sub(start).Hours() / 24) }

func Describe(b byte, r rune, small int8, u uint, f float32) string {
	return fmt.Sprint(b, r, small, u, f)
}

func Divmod(a, b int) (int, int, error) {
	if b == 0 {
		return 0, 0, errors.New("division by zero")
	}
	return a / b, a % b, nil
}

// The param named t must not shadow the fuzz closure's *testing.T.
func Count(data []byte, t int) int { return len(data) + t }
