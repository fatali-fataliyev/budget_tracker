package auth

import (
	"fmt"
	"regexp"
	"time"

	appErrors "github.com/fatali-fataliyev/budget_tracker/customErrors"
)

const (
	MAX_LENGTH_FULLNAME = 255
	MAX_LENGTH_USERNAME = 255
	MAX_LENGTH_EMAIL    = 255
	MAX_PASSWORD_LENGTH = 72
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

type DeleteUser struct {
	Password string
	Reason   string
}

func (newUser NewUser) ValidateUserFields() error {
	usernameRegex := regexp.MustCompile(`^[a-z0-9_]{1,30}$`)
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9](\.?[a-zA-Z0-9_%+-])*@[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)*\.[a-zA-Z]{2,}$`)
	if newUser.UserName == "" {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username cannot be empty!",
		}
	}
	if !usernameRegex.MatchString(newUser.UserName) {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username contains wrong characters, example username: john_doe",
		}
	}
	if len(newUser.FullName) > MAX_LENGTH_FULLNAME {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Username so long, maximum length is %d", MAX_LENGTH_USERNAME),
		}
	}
	if newUser.Email == "" {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Email cannot be empty!",
		}
	}
	if !emailRegex.MatchString(newUser.Email) {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Invalid email format, example valid email: john.doe@gmail.com",
		}
	}
	if len(newUser.Email) > MAX_LENGTH_EMAIL {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Email so long, maximum length is %d", MAX_LENGTH_EMAIL),
		}
	}
	if newUser.PasswordPlain == "" {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Password cannot be empty!",
		}
	}
	if len(newUser.PasswordPlain) > MAX_PASSWORD_LENGTH {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Password so long, maximum length is %d", MAX_PASSWORD_LENGTH),
		}
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
