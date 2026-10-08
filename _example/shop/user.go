package shop

import (
	"errors"
	"strings"
)

type User struct {
	Email    string
	Password string
}

func ValidateEmail(email string) error {
	if !strings.Contains(email, "@") {
		return errors.New("email needs an @")
	}
	return nil
}

func Login(user User, password string) bool { return user.Password == password }
