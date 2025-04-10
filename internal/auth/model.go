package auth

import (
	"fmt"
	"regexp"
	"time"
)

const (
	maxLenForFullName = 255
	maxLenForEmail    = 255
)

type User struct {
	ID             string
	UserName       string
	FullName       string
	NickName       string
	Email          string
	PasswordHashed string
	PendingEmail   string
}

type NewUser struct {
	UserName      string
	FullName      string
	NickName      string
	Email         string
	PasswordPlain string
	PendingEmail  string
}

func (newUser NewUser) Validate() error {
	usernameRegex := regexp.MustCompile(`^[a-z0-9_]{1,30}$`)
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9](\.?[a-zA-Z0-9_%+-])*@[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)*\.[a-zA-Z]{2,}$`)
	if newUser.UserName == "" {
		return fmt.Errorf("username is empty")
	}
	if !usernameRegex.MatchString(newUser.UserName) {
		return fmt.Errorf("invalid username format")
	}

	if newUser.FullName == "" {
		return fmt.Errorf("full name is required")
	}
	if len(newUser.FullName) > 255 {
		return fmt.Errorf("fullname so long, maximum length: %d", maxLenForFullName)
	}
	if newUser.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !emailRegex.MatchString(newUser.Email) {
		return fmt.Errorf("invalid email format")
	}
	if len(newUser.Email) > 255 {
		return fmt.Errorf("your email address so long, maximum length: %d", maxLenForEmail)
	}
	if newUser.PasswordPlain == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

type Session struct {
	ID        string
	Token     string
	CreatedAt time.Time
	ExpireAt  time.Time
	UserID    string
}

type UserCredentials struct {
	UserName       string
	PasswordHashed string
}

type UserCredentialsPure struct {
	UserName      string
	PasswordPlain string
}
