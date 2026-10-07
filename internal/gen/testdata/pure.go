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

// Spreads the remainder over the first parts so they always add up to total.
func Split(total int64, parts int) []int64 {
	if parts <= 0 || parts > 1000 {
		return nil
	}

	share, rest := total/int64(parts), total%int64(parts)
	result := make([]int64, parts)
	for i := range result {
		result[i] = share
		switch {
		case int64(i) < rest:
			result[i]++
		case int64(i) < -rest:
			result[i]--
		}
	}
	return result
}

func WeekRange(day time.Time) (time.Time, time.Time) {
	start := day.AddDate(0, 0, -int(day.Weekday()))
	return start, start.AddDate(0, 0, 7)
}
