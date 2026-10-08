package user

import (
	"context"
	"errors"
	"os"
)

type User struct{ Email string }

func ValidateEmail(s string) error {
	if s == "" {
		return errors.New("empty")
	}
	return nil
}

func IsValidAge(n int) bool { return n >= 0 }

func (u User) Validate() error { return ValidateEmail(u.Email) }

func checkName(s string) bool { return s != "" }

func IsEmail(s string) bool { return s != "" }

// not validation below

func Save(u *User) error { return nil }

func Checkout(id, qty int) error { return nil }

func Ping(ctx context.Context) error { return nil }

func CheckConfig() error { return nil }

func Delete(path string) error { return os.RemoveAll(path) }

func ValidateFile(path string) error {
	_, err := os.Stat(path)
	return err
}

type Box[T any] struct{ value T }

func (b Box[T]) Validate() error { return nil }
