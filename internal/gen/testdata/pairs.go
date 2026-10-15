package fixture

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// FormatCents and ParseCents round-trip: the fuzz test feeds one into the other.
func FormatCents(cents int64) string { return strconv.FormatInt(cents, 10) }

func ParseCents(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }

// EncodeID and DecodeID round-trip bytes, nil and empty included.
func EncodeID(id []byte) string { return hex.EncodeToString(id) }

func DecodeID(s string) ([]byte, error) { return hex.DecodeString(s) }

type Color int

const (
	Red Color = iota
	Green
	Blue
	colorCount // a bound, not a color: left out of the enum test
)

func (c Color) String() string {
	switch c {
	case Red:
		return "red"
	case Green:
		return "green"
	case Blue:
		return "blue"
	}
	return fmt.Sprintf("Color(%d)", int(c))
}

func ParseColor(s string) (Color, error) {
	for c := Red; c < colorCount; c++ {
		if c.String() == s {
			return c, nil
		}
	}
	return 0, fmt.Errorf("unknown color %q", s)
}

// NormalizeTag is idempotent: a normalized tag normalizes to itself.
func NormalizeTag(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }
