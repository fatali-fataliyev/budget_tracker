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
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrAuth         = errors.New("unauthorized")
	ErrAccessDenied = errors.New("access denied")
	ErrConflict     = errors.New("conflict")
)

const (
	limitPerTransaction   = 99999999
	maxLenForCategory     = 255
	maxLenForCurrency     = 30
	maxLenForCategoryName = 255
	maxAmountLimit        = 999999999999999999.99
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
	SaveCategory(category Category) error
	CheckSession(token string) (userId string, err error)
	UpdateSession(userId string, expireAt time.Time) error
	GetSessionByToken(token string) (auth.Session, error)
	SaveTransaction(t Transaction) error
	GetFilteredTransactions(userID string, filters *ListTransactionsFilters) ([]Transaction, error)
	GetTransactionById(userID string, transacationID string) (Transaction, error)
	GetTotals(userId string, filters GetTotals) (GetTotals, error)
	ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error)
	IsUserExists(username string) (bool, error)
	IsEmailConfirmed(emailAddress string) bool
	UpdateTransaction(userID string, transacationItem UpdateTransactionItem) error
	ChangeAmountOfTransaction(userId string, tId string, tType string, amount float64) error
	DeleteTransaction(userID string, transacationID string) error
	LogoutUser(userId string, token string) error
	GetStorageType() string
}

func (bt *BudgetTracker) ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error) {
	user, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return auth.User{}, fmt.Errorf("failed to login: %w", err)
	}
	return user, nil
}

func (bt *BudgetTracker) GenerateSession(credentialsPure auth.UserCredentialsPure) (string, error) {
	user, err := bt.storage.ValidateUser(credentialsPure)
	if err != nil {
		return "", fmt.Errorf("failed to login: %w", err)
	}

	tokenByte := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, tokenByte); err != nil {
		return "", fmt.Errorf("failed to generate new session: %w", err)
	}
	token := hex.EncodeToString(tokenByte)

	now := time.Now()
	session := auth.Session{
		ID:        uuid.New().String(),
		Token:     token,
		CreatedAt: now,
		ExpireAt:  now.AddDate(0, 3, 0),
		UserID:    user.ID,
	}
	err = bt.storage.SaveSession(session)
	if err != nil {
		return "", fmt.Errorf("failed to save session: %w", err)
	}
	return token, nil
}

func (bt *BudgetTracker) CheckSession(token string) (string, error) {
	session, err := bt.storage.GetSessionByToken(token)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return "", err
	}

	now := time.Now()
	daysUntilExpiry := int(session.ExpireAt.Sub(now).Hours() / 24)

	if daysUntilExpiry <= 5 {
		newExpireAt := time.Now().AddDate(0, 1, 0)

		err := bt.storage.UpdateSession(userId, newExpireAt)
		if err != nil {
			return "", fmt.Errorf("failed to update session")
		}
		return userId, nil
	}
	return userId, nil
}

func (bt *BudgetTracker) IsUserExists(username string) (bool, error) {
	result, err := bt.storage.IsUserExists(username)
	if err != nil {
		return false, fmt.Errorf("failed to check user existance: %w", err)
	}
	return result, nil
}

func (bt *BudgetTracker) RegisterUser(newUser auth.NewUser) (string, error) {
	isExists, err := bt.IsUserExists(newUser.UserName)
	if err != nil {
		return "", fmt.Errorf("failed to check username availability: %w", err)
	}
	if isExists {
		return "", fmt.Errorf("%w: this '%s' username already taken", ErrConflict, newUser.UserName)
	}
	if existingEmailAddress := bt.storage.IsEmailConfirmed(newUser.Email); existingEmailAddress != false {
		return "", fmt.Errorf("%w: this: '%s' email address already taken and confirmed, try to register with another email.", ErrConflict, newUser.Email)
	}
	hashedPassword, err := auth.HashPassword(newUser.PasswordPlain)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	user := auth.User{
		ID:             uuid.New().String(),
		UserName:       strings.ToLower(newUser.UserName),
		FullName:       strings.ToUpper(newUser.FullName),
		NickName:       newUser.NickName,
		Email:          strings.ToLower(newUser.Email),
		PasswordHashed: hashedPassword,
		PendingEmail:   strings.ToLower(newUser.Email),
	}

	if err := bt.storage.SaveUser(user); err != nil {
		return "", fmt.Errorf("failed to save user: %w", err)
	}

	credentials := auth.UserCredentialsPure{
		UserName:      newUser.UserName,
		PasswordPlain: newUser.PasswordPlain,
	}

	token, err := bt.GenerateSession(credentials)
	if err != nil {
		return "", fmt.Errorf("failed to generate session: %w | please login", err)
	}
	return token, nil
}

func (bt *BudgetTracker) SaveTransaction(createdBy string, amount float64, limit float64, category string, transcationType string, currency string) error {
	if amount < 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	if math.Abs(amount) < 1e-9 {
		return fmt.Errorf("%w: zero value amount", ErrInvalidInput)
	}
	if amount > limitPerTransaction {
		return fmt.Errorf("%w: amount exceeds maximum value: max:%d, entered:%2.f", ErrInvalidInput, limitPerTransaction, amount)
	}
	if limit != 0 {
		if amount >= limit {
			return fmt.Errorf("limit must be greater than amount")
		}
	}
	if strings.TrimSpace(transcationType) != "+" && strings.TrimSpace(transcationType) != "-" {
		return fmt.Errorf("%w: allowed transaction types are: income(+) and expense(-)", ErrInvalidInput)
	}
	if len(category) > maxLenForCategory {
		return fmt.Errorf("%w: category name too long, maximum length: %d", ErrInvalidInput, maxLenForCategory)
	}
	if len(currency) > maxLenForCurrency {
		return fmt.Errorf("%w: currency name too long, maximum length: %d", ErrInvalidInput, maxLenForCurrency)
	}

	now := time.Now()
	t := Transaction{
		ID:          uuid.New().String(),
		Amount:      amount,
		Limit:       limit,
		Currency:    strings.ToLower(currency),
		Category:    strings.ToLower(category),
		UpdatedDate: now,
		CreatedDate: now,
		Type:        transcationType,
		CreatedBy:   createdBy,
	}

	if err := bt.storage.SaveTransaction(t); err != nil {
		return fmt.Errorf("failed to save transaction to db: %w", err)
	}
	return nil
}

func (bt *BudgetTracker) SaveCategory(userId string, name string, cType string, maxAmount float64, periodDays int) error {
	if maxAmount > maxAmountLimit {
		return fmt.Errorf("%w: max amount is too large; the limit is: %.2f", ErrInvalidInput, maxAmountLimit)
	}
	if len(name) > maxLenForCategoryName {
		return fmt.Errorf("%w: name is too long for category; the limit is: %d", ErrInvalidInput, maxLenForCategoryName)
	}

	var category Category
	cTypeLower := strings.ToLower(cType)
	if cTypeLower != "+" && cTypeLower != "-" {
		return fmt.Errorf("%w: allowed category types are: income(+), expense(-).", ErrInvalidInput)
	}
	if cTypeLower == "-" {
		now := time.Now()
		category = Category{
			ID:          uuid.New().String(),
			Name:        name,
			Type:        cTypeLower,
			CreatedDate: now,
			UpdatedDate: now,
			MaxAmount:   maxAmount,
			PeriodDays:  periodDays,
			CreatedBy:   userId,
		}
	} else {
		now := time.Now()
		category = Category{
			ID:          uuid.New().String(),
			Name:        name,
			Type:        cTypeLower,
			CreatedDate: now,
			UpdatedDate: now,
			MaxAmount:   0,
			PeriodDays:  0,
			CreatedBy:   userId,
		}
	}

	if err := bt.storage.SaveCategory(category); err != nil {
		return fmt.Errorf("failed to save category to db: %w", err)
	}
	return nil

}

func (bt *BudgetTracker) GetFilteredTransactions(userId string, filters *ListTransactionsFilters) ([]Transaction, error) {
	ts, err := bt.storage.GetFilteredTransactions(userId, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	return ts, nil
}

func (bt *BudgetTracker) GetTotals(userId string, filters GetTotals) (GetTotals, error) {
	result, err := bt.storage.GetTotals(userId, filters)
	if err != nil {
		return GetTotals{}, fmt.Errorf("failed to get totals: %w", err)
	}
	return result, nil
}

func (bt *BudgetTracker) GetTranscationById(userId string, transactionId string) (Transaction, error) {
	t, err := bt.storage.GetTransactionById(userId, transactionId)
	if err != nil {
		return Transaction{}, fmt.Errorf("failed to get transaction by id: %w", err)
	}
	return t, nil
}

func (bt *BudgetTracker) UpdateTransaction(userId string, updateTItem UpdateTransactionItem) error {
	if updateTItem.Amount < 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	if math.Abs(updateTItem.Amount) < 1e-9 {
		return fmt.Errorf("%w: zero value amount", ErrInvalidInput)
	}
	if updateTItem.Amount > limitPerTransaction {
		return fmt.Errorf("%w: amount exceeds maximum value: max:%d, entered:%2.f", ErrInvalidInput, limitPerTransaction, updateTItem.Amount)
	}
	if updateTItem.Amount >= updateTItem.Limit {
		return fmt.Errorf("limit must be greater than amount")
	}
	if updateTItem.Type != "+" && updateTItem.Type != "-" {
		return fmt.Errorf("%w: allowed transaction types are: income(+) and expense(-)", ErrInvalidInput)
	}
	if len(updateTItem.Category) > maxLenForCategory {
		return fmt.Errorf("%w: category name too long, maximum length: %d", ErrInvalidInput, maxLenForCategory)
	}
	if len(updateTItem.Currency) > maxLenForCurrency {
		return fmt.Errorf("%w: currency name too long, maximum length: %d", ErrInvalidInput, maxLenForCurrency)
	}
	tItem, err := bt.storage.GetTransactionById(userId, updateTItem.ID)
	if err != nil {
		return fmt.Errorf("failed to get transaction's creator: %w", err)
	}
	if userId != tItem.CreatedBy {
		return fmt.Errorf("%w: you are not allowed to update a transaction you did not create", ErrAccessDenied)
	}
	if err := bt.storage.UpdateTransaction(userId, updateTItem); err != nil {
		return fmt.Errorf("failed to update transaction, Transaction-ID: %s, error: %w", updateTItem.ID, err)
	}
	return nil
}

func (bt *BudgetTracker) ChangeAmountOfTransaction(userId string, tId string, tType string, amount float64) error {
	t, err := bt.storage.GetTransactionById(userId, tId)
	if err != nil {
		return fmt.Errorf("failed to change amount of transaction: %w", err)
	}

	if tType == "expense" {
		tType = "-"
	} else if tType == "income" {
		tType = "+"
	} else {
		return fmt.Errorf("%w: invalid transaction type", ErrInvalidInput)
	}
	if amount > limitPerTransaction {
		return fmt.Errorf("%w: amount exceeds maximum value: max:%d, entered:%2.f", ErrInvalidInput, limitPerTransaction, amount)
	}
	if amount > t.Limit {
		return fmt.Errorf("%w: amount cannot be greater than the limit, limit for this transaction is: %2.f", ErrInvalidInput, t.Limit)
	}
	err = bt.storage.ChangeAmountOfTransaction(userId, tId, tType, amount)
	if err != nil {
		return fmt.Errorf("failed to change amount of transaction: %w", err)
	}
	return nil
}

func (bt *BudgetTracker) DeleteTransaction(userId string, transactionId string) error {
	tItem, err := bt.storage.GetTransactionById(userId, transactionId)
	if err != nil {
		return fmt.Errorf("failed to get transaction's creator: %w", err)
	}
	if userId != tItem.CreatedBy {
		return fmt.Errorf("%w: you are not allowed to delete a transaction you did not create", ErrAccessDenied)
	}
	if err := bt.storage.DeleteTransaction(userId, transactionId); err != nil {
		return fmt.Errorf("failed to delete transaction, Transaction-ID: %s, error: %w", transactionId, err)
	}
	return nil
}

func (bt *BudgetTracker) LogoutUser(userId string, token string) error {
	err := bt.storage.LogoutUser(userId, token)
	if err != nil {
		return err
	}
	return nil
}
