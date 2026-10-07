package auth

import "go/token"

type User struct {
	Password string
	Token    string
}

func Login(user User, password string) bool { return user.Password == password }

func Valid(sessionID, stored string) bool { return sessionID != stored }

// not flagged below

func HasToken(user User) bool { return user.Token != "" }

func Missing(session *User) bool { return session == nil }

func IsEOF(tok token.Token) bool { return tok == token.EOF }

func Retries(tokenCount int) bool { return tokenCount == 3 }
