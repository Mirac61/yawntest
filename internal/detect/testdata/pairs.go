package fixture

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Roundtrip: same suffix, matching types, both pure.
func FormatCents(cents int64) string { return strconv.FormatInt(cents, 10) }

func ParseCents(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }

func EncodeID(id []byte) string { return hex.EncodeToString(id) }

func DecodeID(s string) ([]byte, error) { return hex.DecodeString(s) }

// No roundtrip: floats round, so "same value back" is a matter of taste.
func FormatRatio(r float64) string { return strconv.FormatFloat(r, 'f', 2, 64) }

func ParseRatio(s string) (float64, error) { return strconv.ParseFloat(s, 64) }

// No roundtrip: the decode half reads a file.
func EncodeConfig(name string) string { return name + ".json" }

func DecodeConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

// No roundtrip: the types don't line up.
func FormatCount(n int) string { return strconv.Itoa(n) }

func ParseCount(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }

type Color int

const (
	ColorUnknown Color = iota // a sentinel, left out
	Red
	Green
	numColors // a bound, left out
)

const Blue Color = 3

func (c Color) String() string {
	return [...]string{"unknown", "red", "green", "blue"}[c]
}

func ParseColor(s string) (Color, error) {
	for c := ColorUnknown; c <= Blue; c++ {
		if c.String() == s {
			return c, nil
		}
	}
	return 0, fmt.Errorf("unknown color %q", s)
}

// No enum roundtrip: String has a pointer receiver, so Size(1).String() doesn't compile.
type Size int

const Small Size = 1

func (s *Size) String() string { return "small" }

func ParseSize(s string) (Size, error) { return Small, nil }

// Idempotent: one string in, the same type out, named like a normalizer.
func NormalizeTag(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }

// Not pure, because its helper writes to stdout.
func Shout(s string) string { return loud(s) }

func loud(s string) string {
	fmt.Println(s)
	return strings.ToUpper(s)
}
