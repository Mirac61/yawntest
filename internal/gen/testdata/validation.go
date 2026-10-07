package fixture

import (
	"errors"
	"strings"
	"time"
)

type User struct{ Email string }

type Slug string

func ValidateEmail(s string) error {
	if !strings.Contains(s, "@") {
		return errors.New("missing @")
	}
	return nil
}

func IsValidAge(n int) bool { return n >= 0 && n < 150 }

func (u User) Validate() error { return ValidateEmail(u.Email) }

func ValidateUser(u *User) error {
	if u == nil {
		return errors.New("nil user")
	}
	return u.Validate()
}

func CheckSlug(s Slug) bool { return s != "" }

// Detected by its signature only, so no "empty is rejected" test.
func IsEmail(s string) bool { return strings.Contains(s, "@") }

func ValidateTags(tags []string) error {
	if len(tags) > 10 {
		return errors.New("too many tags")
	}
	return nil
}

func IsValidDeadline(deadline time.Time) bool { return deadline.After(time.Unix(0, 0)) }

func IsValidName(name string) bool { return strings.TrimSpace(name) != "" }
