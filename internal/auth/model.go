package auth

import (
	"fmt"
	"regexp"
	"time"

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"
)

const (
	MAX_LENGTH_FULLNAME = 255
	MAX_LENGTH_USERNAME = 255
	MAX_LENGTH_EMAIL    = 255
)

type User struct {
	ID             string
	UserName       string
	FullName       string
	PasswordHashed string
	Email          string
	PendingEmail   string
}

type NewUser struct {
	UserName      string
	FullName      string
	PasswordPlain string
	Email         string
}

func (newUser NewUser) ValidateUserFields() error {
	usernameRegex := regexp.MustCompile(`^[a-z0-9_]{1,30}$`)
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9](\.?[a-zA-Z0-9_%+-])*@[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)*\.[a-zA-Z]{2,}$`)
	if newUser.UserName == "" {
		return fmt.Errorf("%w: username is empty", appErrors.ErrInvalidInput)
	}
	if !usernameRegex.MatchString(newUser.UserName) {
		return fmt.Errorf("%w: this '%s' username is invalid, example valid username: john_doe", appErrors.ErrInvalidInput, newUser.UserName)
	}
	if len(newUser.FullName) > MAX_LENGTH_FULLNAME {
		return fmt.Errorf("%w: fullname so long, maximum length: %d", appErrors.ErrInvalidInput, MAX_LENGTH_FULLNAME)
	}
	if newUser.Email == "" {
		return fmt.Errorf("%w: email is required", appErrors.ErrInvalidInput)
	}
	if !emailRegex.MatchString(newUser.Email) {
		return fmt.Errorf("%w: invalid email format, example valid email: john.doe@gmail.com", appErrors.ErrInvalidInput)
	}
	if len(newUser.Email) > MAX_LENGTH_EMAIL {
		return fmt.Errorf("%w: your email address so long, maximum length: %d", appErrors.ErrInvalidInput, MAX_LENGTH_EMAIL)
	}
	if newUser.PasswordPlain == "" {
		return fmt.Errorf("%w: password is required", appErrors.ErrInvalidInput)
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
