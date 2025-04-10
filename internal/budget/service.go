package budget

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/google/uuid"
)

var (
	errNotFound     = errors.New("not found")
	errInvalidInput = errors.New("invalid input")
	errAuth         = errors.New("unauthorized")
	errAccessDenied = errors.New("access denied")
	errConflict     = errors.New("conflict")
)

const (
	limitPerTransaction = 99999999
	maxLenForCategory   = 255
	maxLenForCurrency   = 30
)

type BudgetTracker struct {
	storage     Storage
	StorageType string
}

func NewBudgetTracker(s Storage) BudgetTracker {
	return BudgetTracker{
		storage:     s,
		StorageType: s.GetStorageType(),
	}

}

type Storage interface {
	SaveUser(newUser auth.User) error
	SaveSession(session auth.Session) error
	CheckSession(token string) (userId string, err error)
	SaveTransaction(t Transaction) error
	GetAllTransactions(userID string) ([]Transaction, error)
	GetTransactionsByType(userID string, transactionType string) ([]Transaction, error)
	GetTransactionsByCategory(userID string, category string) ([]Transaction, error)
	GetTransactionById(userID string, transacationID string) (Transaction, error)
	GetTransactionsByCurrency(userID string, currencyType string) ([]Transaction, error)
	GetTotalsByType(tType string, userID string) (string, error)
	ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error)
	FindUserByUserName(username string) (string, error)
	IsEmailConfirmed(emailAddress string) bool
	UpdateTransaction(userID string, transacationItem UpdateTransactionItem) error
	DeleteTransaction(userID string, transacationID string) error
	GetStorageType() string
}

func (bt *BudgetTracker) GenerateSession(credentialsPure auth.UserCredentialsPure) (string, error) {
	user, err := bt.storage.ValidateUser(credentialsPure)
	if err != nil && user == (auth.User{}) {
		return "", fmt.Errorf("failed to login: %w", err)
	}

	tokenByte := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, tokenByte); err != nil {
		return "", fmt.Errorf("failed to generate new session: %w", err)
	}
	token := hex.EncodeToString(tokenByte)

	now := time.Now()
	session := auth.Session{
		ID:        uuid.New().String(),
		Token:     token,
		CreatedAt: now,
		ExpireAt:  now.AddDate(0, 9, 0),
		UserID:    user.ID,
	}
	err = bt.storage.SaveSession(session)
	if err != nil {
		return "", fmt.Errorf("failed to save session: %w", err)
	}
	return fmt.Sprintf("token:%s", token), nil
}

func (bt *BudgetTracker) SaveUser(newUser auth.NewUser) error {
	takenUserName, err := bt.storage.FindUserByUserName(newUser.UserName)
	if err != nil && takenUserName != "" {
		return fmt.Errorf("%w: this: '%s' username already taken", errConflict, newUser.UserName)
	}
	if existingEmailAddress := bt.storage.IsEmailConfirmed(newUser.Email); existingEmailAddress != false {
		return fmt.Errorf("%w: this: '%s' email address already taken and confirmed, try to register with another email.", errConflict, newUser.Email)
	}

	hashedPassword, err := auth.HashPassword(newUser.PasswordPlain)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := auth.User{
		ID:             uuid.New().String(),
		UserName:       newUser.UserName,
		FullName:       newUser.FullName,
		NickName:       newUser.NickName,
		Email:          newUser.Email,
		PasswordHashed: hashedPassword,
		PendingEmail:   newUser.Email,
	}

	if err := bt.storage.SaveUser(user); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	return nil
}

func (bt *BudgetTracker) SaveTransaction(token string, amount float64, category string, transcationType string, currency string) error {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return fmt.Errorf("%s:  failed to save transaction: %w", errAuth, err)
	}

	if amount < 0 {
		return fmt.Errorf("%w: amount must be positive", errInvalidInput)
	}
	if math.Abs(amount) < 1e-9 {
		return fmt.Errorf("%w: zero value amount", errInvalidInput)
	}
	if amount > limitPerTransaction {
		return fmt.Errorf("%w: amount exceeds maximum value: max:%d, entered:%f", errInvalidInput, limitPerTransaction, amount)
	}
	if strings.TrimSpace(transcationType) != "+" && strings.TrimSpace(transcationType) != "-" {
		return fmt.Errorf("%w: allowed transaction types are: income(+) and expense(-)", errInvalidInput)
	}
	if len(category) > maxLenForCategory {
		return fmt.Errorf("%w: category name too long, maximum length: %d", errInvalidInput, maxLenForCategory)
	}
	if len(currency) > maxLenForCurrency {
		return fmt.Errorf("%w: currency name too long, maximum length: %d", errInvalidInput, maxLenForCurrency)
	}

	now := time.Now()

	t := Transaction{
		ID:          uuid.New().String(),
		Amount:      amount,
		Currency:    strings.ToLower(currency),
		Category:    strings.ToLower(category),
		UpdatedDate: now,
		CreatedDate: now,
		Type:        transcationType,
		CreatedBy:   userId,
	}

	if err := bt.storage.SaveTransaction(t); err != nil {
		return fmt.Errorf("failed to save transaction to db: %w", err)
	}
	return nil
}

func (bt *BudgetTracker) GetAllTransactions(token string) ([]Transaction, error) {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return nil, fmt.Errorf("%w:  failed to get transaction: %w", errAuth, err)
	}

	ts, err := bt.storage.GetAllTransactions(userId)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get transactions: %w", errAuth, err)
	}
	return ts, nil
}

func (bt *BudgetTracker) GetTransactionsByType(token string, transactionType string) ([]Transaction, error) {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return nil, fmt.Errorf("%w:  failed to get transactions by type: %w", errAuth, err)
	}

	results, err := bt.storage.GetTransactionsByType(userId, transactionType)
	if err != nil {
		return []Transaction{}, fmt.Errorf("failed to get transactions by type: %w", err)
	}
	return results, nil
}

func (bt *BudgetTracker) GetTransactionsByCategory(token string, category string) ([]Transaction, error) {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return nil, fmt.Errorf("%w:  failed to get transactions by type: %w", errAuth, err)
	}

	ts, err := bt.storage.GetTransactionsByCategory(userId, strings.ToLower(category))
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by category: %w", err)
	}
	return ts, nil

}

func (bt *BudgetTracker) GetTranscationById(token string, transactionId string) (Transaction, error) {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return Transaction{}, fmt.Errorf("%w:  failed to get transaction by id: %w", errAuth, err)
	}
	t, err := bt.storage.GetTransactionById(userId, transactionId)
	if err != nil {
		return Transaction{}, fmt.Errorf("failed to get transaction by id: %w", err)
	}
	return t, nil
}

func (bt *BudgetTracker) GetTotalsByType(tType string, token string) (string, error) {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return "", fmt.Errorf("%w:  failed to get transactions by type: %w", errAuth, err)
	}
	results, err := bt.storage.GetTotalsByType(tType, userId)
	if err != nil {
		return "", fmt.Errorf("failed to get transactions by type: %w", err)
	}
	return results, nil
}

func (bt *BudgetTracker) UpdateTransaction(token string, updateTItem UpdateTransactionItem) error {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return fmt.Errorf("%w:  failed to update transaction: %w", errAuth, err)
	}
	if updateTItem.Amount < 0 {
		return fmt.Errorf("%w: amount must be positive", errInvalidInput)
	}
	if math.Abs(updateTItem.Amount) < 1e-9 {
		return fmt.Errorf("%w: zero value amount", errInvalidInput)
	}
	if updateTItem.Amount > limitPerTransaction {
		return fmt.Errorf("%w: amount exceeds maximum value: max:%d, entered:%f", errInvalidInput, limitPerTransaction, updateTItem.Amount)
	}
	if updateTItem.Type != "+" && updateTItem.Type != "-" {
		return fmt.Errorf("%w: allowed transaction types are: income(+) and expense(-)", errInvalidInput)
	}
	if len(updateTItem.Category) > maxLenForCategory {
		return fmt.Errorf("%w: category name too long, maximum length: %d", errInvalidInput, maxLenForCategory)
	}
	if len(updateTItem.Currency) > maxLenForCurrency {
		return fmt.Errorf("%w: currency name too long, maximum length: %d", errInvalidInput, maxLenForCurrency)
	}
	tItem, err := bt.storage.GetTransactionById(userId, updateTItem.ID)
	if err != nil {
		return fmt.Errorf("failed to get transaction's creator: %w", err)
	}
	if userId != tItem.CreatedBy {
		return fmt.Errorf("%w: you cannot update other's transaction.", errAccessDenied)
	}
	if err := bt.storage.UpdateTransaction(userId, updateTItem); err != nil {
		return fmt.Errorf("failed to update transaction, Transaction-ID: %s, error: %w", updateTItem.ID, err)
	}
	return nil
}

func (bt *BudgetTracker) DeleteTransaction(token string, transactionId string) error {
	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return fmt.Errorf("%w:  failed to delete transaction: %w", errAuth, err)
	}
	tItem, err := bt.storage.GetTransactionById(userId, transactionId)
	if err != nil {
		return fmt.Errorf("failed to get transaction's creator: %w", err)
	}
	if userId != tItem.CreatedBy {
		return fmt.Errorf("%w: you cannot delete other's transaction.", errAccessDenied)
	}
	if err := bt.storage.DeleteTransaction(userId, transactionId); err != nil {
		return fmt.Errorf("failed to delete transaction, Transaction-ID: %s, error: %w", transactionId, err)
	}
	return nil
}
