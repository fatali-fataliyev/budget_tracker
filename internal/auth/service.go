package auth

import (
	"context"
	"fmt"

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"
	"github.com/fatali-fataliyev/budget_tracker/internal/contextutil"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(ctx context.Context, password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	traceID := contextutil.TraceIDFromContext(ctx)
	if err != nil {
		logging.Logger.Errorf("[Trace-ID: %s] | failed to hash user password in HashPassword() function | Error : %v", traceID, err)

		return "", appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}
	return string(hashedPassword), nil
}

func ComparePasswords(hashedPwd string, plainPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))
	return err == nil
}
