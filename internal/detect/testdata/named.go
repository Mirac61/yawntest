package fixture

import (
	"strconv"
	"strings"
)

// Types of the package defined as a basic type: fuzzed as that type, converted for the call.
type (
	Handle string
	Amount int64
	Level  uint8
)

func Shout(h Handle) Handle { return Handle(strings.ToUpper(string(h))) }

// Amount(l)+1, not Amount(l+1): l+1 overflows uint8 at 255, which the generated fuzz test
// found while this fixture was written.
func Halve(a Amount, l Level) Amount { return a / (Amount(l) + 1) }

// Idempotent even though its type is Handle and not string.
func NormalizeHandle(h Handle) Handle { return Handle(strings.ToLower(strings.TrimSpace(string(h)))) }

// A roundtrip over Amount.
func FormatAmount(a Amount) string { return strconv.FormatInt(int64(a), 10) }

func ParseAmount(s string) (Amount, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	return Amount(n), err
}

func ValidateHandle(h Handle) error {
	if h == "" {
		return errEmptyHandle
	}
	return nil
}

var errEmptyHandle = strconvError("empty handle")

type strconvError string

func (e strconvError) Error() string { return string(e) }

// Its fields of type Handle and Amount get values in the JSON roundtrip.
type Account struct {
	Owner   Handle `json:"owner"`
	Balance Amount `json:"balance"`
}
