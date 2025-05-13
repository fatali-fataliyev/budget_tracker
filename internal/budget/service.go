package budget

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/google/uuid"
)

const (
	MAX_TRANSACTION_AMOUNT_LIMIT         = 999999999999999999
	MAX_TRANSACTION_CURRENCY_LENGTH      = 255
	MAX_TRANSACTION_NOTE_LENGTH          = 1000
	MAX_TRANSACTION_CATEGORY_NAME_LENGTH = 255
	MAX_CATEGORY_AMOUNT_LIMIT            = 999999999999999999.99
	MAX_CATEGORY_NAME_LENGTH             = 255
	MAX_TARGET_AMOUNT_LIMIT              = 999999999999999999
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
	SaveExpenseCategory(category ExpenseCategory) error
	SaveIncomeCategory(category IncomeCategory) error
	CheckSession(token string) (userId string, err error)
	UpdateSession(userId string, expireAt time.Time) error
	GetSessionByToken(token string) (auth.Session, error)
	SaveTransaction(t Transaction) error
	// GetFilteredTransactions(userID string) ([]Transaction, error)
	GetFilteredExpenseCategories(userID string, filters *ExpenseCategoryList) ([]ExpenseCategoryResponse, error)
	GetTransactionById(userID string, transacationID string) (Transaction, error)
	GetTotals(userId string, filters GetTotals) (GetTotals, error)
	ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error)
	IsUserExists(username string) (bool, error)
	IsEmailConfirmed(emailAddress string) bool
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
		return "", fmt.Errorf("%w", err)
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
		return "", fmt.Errorf("%w", err)
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

func (bt *BudgetTracker) SaveUser(newUser auth.NewUser) (string, error) {
	isExists, err := bt.IsUserExists(newUser.UserName)
	if err != nil {
		return "", fmt.Errorf("failed to check username availability: %w", err)
	}
	if isExists {
		return "", fmt.Errorf("%w: this '%s' username already taken", appErrors.ErrConflict, newUser.UserName)
	}
	if existingEmailAddress := bt.storage.IsEmailConfirmed(newUser.Email); existingEmailAddress != false {
		return "", fmt.Errorf("%w: this: '%s' email address already taken and confirmed, try to register with another email.", appErrors.ErrConflict, newUser.Email)
	}
	hashedPassword, err := auth.HashPassword(newUser.PasswordPlain)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	user := auth.User{
		ID:             uuid.New().String(),
		UserName:       strings.ToLower(newUser.UserName),
		FullName:       CapitalizeFullName(newUser.FullName),
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

func CapitalizeFullName(name string) string {
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) == 0 {
			continue
		}
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func (bt *BudgetTracker) SaveTransaction(userId string, transaction TransactionRequest) error {
	if transaction.Amount == 0 {
		return fmt.Errorf("%w: minimum amount must be 1.", appErrors.ErrInvalidInput)
	}
	if transaction.Amount > MAX_TRANSACTION_AMOUNT_LIMIT {
		return fmt.Errorf("%w: maximum allowed amount per transaction is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_AMOUNT_LIMIT)
	}
	if len(transaction.CategoryName) > MAX_TRANSACTION_CATEGORY_NAME_LENGTH {
		return fmt.Errorf("%w: category name so long.", appErrors.ErrInvalidInput)
	}
	if len(transaction.Currency) > MAX_TRANSACTION_CURRENCY_LENGTH {
		return fmt.Errorf("%w: currency so long", appErrors.ErrInvalidInput)
	}
	if len(transaction.Note) > MAX_TRANSACTION_NOTE_LENGTH {
		return fmt.Errorf("%w: note so long, maximum allowed length is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_NOTE_LENGTH)

	}

	now := time.Now().UTC()
	t := Transaction{
		ID:           uuid.New().String(),
		CategoryName: transaction.CategoryName,
		Amount:       transaction.Amount,
		Currency:     transaction.Currency,
		CreatedAt:    now,
		Note:         transaction.Note,
		CreatedBy:    userId,
	}

	if err := bt.storage.SaveTransaction(t); err != nil {
		return fmt.Errorf("failed to save transaction to db: %w", err)
	}
	return nil
}

func (bt *BudgetTracker) SaveExpenseCategory(userId string, category ExpenseCategoryRequest) error {
	if category.MaxAmount > MAX_CATEGORY_AMOUNT_LIMIT {
		return fmt.Errorf("%w: category max amount is too large; the limit is: %.2f", appErrors.ErrInvalidInput, MAX_CATEGORY_AMOUNT_LIMIT)
	}
	if len(category.Name) > MAX_CATEGORY_NAME_LENGTH {
		return fmt.Errorf("%w: category name is too long for category; the limit is: %d", appErrors.ErrInvalidInput, MAX_CATEGORY_NAME_LENGTH)
	}

	now := time.Now().UTC()
	categoryItem := ExpenseCategory{
		ID:        uuid.New().String(),
		Name:      category.Name,
		MaxAmount: category.MaxAmount,
		PeriodDay: category.PeriodDay,
		CreatedAt: now,
		UpdatedAt: now,
		Note:      category.Note,
		CreatedBy: userId,
		Type:      category.Type,
	}

	if err := bt.storage.SaveExpenseCategory(categoryItem); err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) SaveIncomeCategory(userId string, category IncomeCategoryRequest) error {
	if category.TargetAmount > MAX_TARGET_AMOUNT_LIMIT {
		return fmt.Errorf("%w: category max amount is too large; the limit is: %.2f", appErrors.ErrInvalidInput, MAX_CATEGORY_AMOUNT_LIMIT)
	}
	if len(category.Name) > MAX_CATEGORY_NAME_LENGTH {
		return fmt.Errorf("%w: category name is too long for category; the limit is: %d", appErrors.ErrInvalidInput, MAX_CATEGORY_NAME_LENGTH)
	}

	now := time.Now().UTC()
	categoryItem := IncomeCategory{
		ID:           uuid.New().String(),
		Name:         category.Name,
		TargetAmount: category.TargetAmount,
		CreatedAt:    now,
		UpdatedAt:    now,
		Note:         category.Note,
		CreatedBy:    userId,
		Type:         category.Type,
	}

	if err := bt.storage.SaveIncomeCategory(categoryItem); err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) GetFilteredTransactions(userId string) ([]Transaction, error) {
	// ts, err := bt.storage.GetFilteredTransactions(userId)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get transactions: %w", err)
	// }
	// return ts, nil
	return nil, nil
}

func (bt *BudgetTracker) GetFilteredExpenseCategories(userID string, filters *ExpenseCategoryList) ([]ExpenseCategoryResponse, error) {
	categoriesRaw, err := bt.storage.GetFilteredExpenseCategories(userID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense categories: %w", err)
	}

	var categories []ExpenseCategoryResponse

	for _, category := range categoriesRaw {
		var usagePercent int
		if category.MaxAmount > 0 {
			usagePercent = int((category.Amount / category.MaxAmount) * 100)
		}

		isExpired := time.Now().UTC().After(category.CreatedAt.AddDate(0, 0, category.PeriodDay))

		category := ExpenseCategoryResponse{
			ID:           category.ID,
			Name:         category.Name,
			Amount:       category.Amount,
			MaxAmount:    category.MaxAmount,
			PeriodDay:    category.PeriodDay,
			UsagePercent: usagePercent,
			CreatedAt:    category.CreatedAt,
			UpdatedAt:    category.UpdatedAt,
			Note:         category.Note,
			CreatedBy:    category.CreatedBy,
			IsExpired:    isExpired,
		}

		categories = append(categories, category)
	}

	return categories, nil
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

func (bt *BudgetTracker) ChangeAmountOfTransaction(userId string, tId string, tType string, amount float64) error {
	_, err := bt.storage.GetTransactionById(userId, tId)
	if err != nil {
		return fmt.Errorf("failed to change amount of transaction: %w", err)
	}

	if tType == "expense" {
		tType = "-"
	} else if tType == "income" {
		tType = "+"
	} else {
		return fmt.Errorf("%w: invalid transaction type", appErrors.ErrInvalidInput)
	}
	if amount > MAX_TRANSACTION_AMOUNT_LIMIT {
		return fmt.Errorf("%w: amount exceeds maximum value: max:%d, entered:%2.f", appErrors.ErrInvalidInput, MAX_TRANSACTION_AMOUNT_LIMIT, amount)
	}
	if amount > 300 {
		return fmt.Errorf("%w: amount cannot be greater than the limit, limit for this transaction is: %2.f", appErrors.ErrInvalidInput, 300)
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
		return fmt.Errorf("%w: you are not allowed to delete a transaction you did not create", appErrors.ErrAccessDenied)
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
