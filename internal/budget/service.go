package budget

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/logging"
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
	Epsilon                              = 1e-9 // For IsFloatZero() func.
)

func IsFloatZero(f float64) bool {
	return f >= 0 && f < Epsilon
}

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
	GetFilteredTransactions(userID string, filters *TransactionList) ([]Transaction, error)
	GetFilteredExpenseCategories(userID string, filters *ExpenseCategoryList) ([]ExpenseCategoryResponse, error)
	GetFilteredIncomeCategories(userID string, filters *IncomeCategoryList) ([]IncomeCategoryResponse, error)
	GetTransactionById(userID string, transacationID string) (Transaction, error)
	ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error)
	IsUserExists(username string) (bool, error)
	IsEmailConfirmed(emailAddress string) (bool, error)
	UpdateExpenseCategory(userId string, fields UpdateExpenseCategoryRequest) (*ExpenseCategoryResponse, error)
	DeleteExpenseCategory(userId string, categoryId string) error
	DeleteIncomeCategory(userId string, categoryId string) error
	UpdateIncomeCategory(userId string, fields UpdateIncomeCategoryRequest) (*IncomeCategoryResponse, error)
	LogoutUser(userId string, token string) error
	GetStorageType() string
}

func (bt *BudgetTracker) ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error) {
	user, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return auth.User{}, fmt.Errorf("failed to validate user: %w", err)
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

	now := time.Now().UTC()

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
		return "", fmt.Errorf("failed to get session by token: %w", err)
	}

	userId, err := bt.storage.CheckSession(token)
	if err != nil {
		return "", fmt.Errorf("failed to check session: %w", err)
	}

	now := time.Now().UTC()
	daysUntilExpiry := int(session.ExpireAt.Sub(now).Hours() / 24)

	if daysUntilExpiry <= 5 {
		newExpireAt := time.Now().AddDate(0, 1, 0)

		err := bt.storage.UpdateSession(userId, newExpireAt)
		if err != nil {
			return "", fmt.Errorf("failed to update session: %w", err)
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
	isUserExists, err := bt.IsUserExists(newUser.UserName)
	if err != nil {
		return "", fmt.Errorf("failed to check username availability: %w", err)
	}
	if isUserExists {
		return "", fmt.Errorf("%w: this '%s' username already taken", appErrors.ErrConflict, newUser.UserName)
	}
	isEmailTaken, err := bt.storage.IsEmailConfirmed(newUser.Email)
	if err != nil {
		return "", fmt.Errorf("failed to check email availability: %w", err)
	}
	if isEmailTaken {
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
		return "", fmt.Errorf("failed to registration: %w", err)
	}

	credentials := auth.UserCredentialsPure{
		UserName:      newUser.UserName,
		PasswordPlain: newUser.PasswordPlain,
	}

	token, err := bt.GenerateSession(credentials)
	if err != nil {
		return "", fmt.Errorf("registration successfully but failed to generate session: %w | try login", err)
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
	if IsFloatZero(transaction.Amount) {
		return fmt.Errorf("%w: transaction amount is zero or very close to zero.", appErrors.ErrInvalidInput)
	}
	if transaction.Amount > MAX_TRANSACTION_AMOUNT_LIMIT {
		return fmt.Errorf("%w: maximum allowed amount per transaction is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_AMOUNT_LIMIT)
	}
	if len(transaction.CategoryName) > MAX_TRANSACTION_CATEGORY_NAME_LENGTH {
		return fmt.Errorf("%w: category name so long.", appErrors.ErrInvalidInput)
	}
	if len(transaction.Currency) > MAX_TRANSACTION_CURRENCY_LENGTH {
		return fmt.Errorf("%w: currency so long, maximum allowed currency length is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_CURRENCY_LENGTH)
	}
	if len(transaction.Note) > MAX_TRANSACTION_NOTE_LENGTH {
		return fmt.Errorf("%w: note so long, maximum allowed length is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_NOTE_LENGTH)
	}
	if transaction.CategoryType != "+" && transaction.CategoryType != "-" {
		return fmt.Errorf("%w: invalid category type: %s", appErrors.ErrInvalidInput, transaction.CategoryType)
	}

	now := time.Now().UTC()
	txn := Transaction{
		ID:           uuid.New().String(),
		CategoryName: transaction.CategoryName,
		CategoryType: transaction.CategoryType,
		Amount:       transaction.Amount,
		Currency:     transaction.Currency,
		CreatedAt:    now,
		Note:         transaction.Note,
		CreatedBy:    userId,
	}

	if err := bt.storage.SaveTransaction(txn); err != nil {
		return fmt.Errorf("failed to save transaction to db: %w", err)
	}
	return nil
}

func (bt *BudgetTracker) ProcessImage(imageRawText string) (ProcessedImageResponse, error) {
	if imageRawText == "" {
		return ProcessedImageResponse{}, fmt.Errorf("empty raw text")
	}

	var result ProcessedImageResponse

	amountRegex := regexp.MustCompile(`\d+(\.\d+)?`)

	amountMatches := amountRegex.FindAllString(imageRawText, -1)

	for _, amount := range amountMatches {
		num, err := strconv.ParseFloat(amount, 64)
		if err == nil {
			result.Amounts = append(result.Amounts, num)
		}
		logging.Logger.Warn("failed to convert string number to float64 from ProcessImage() function, error: %v", err)
	}

	symbolRegex := regexp.MustCompile(`[$€£¥₹₽₩₪₫₴₦₵₲₺₡₨៛฿₸]`)
	symbolMatches := symbolRegex.FindAllString(imageRawText, -1)

	for _, symbol := range symbolMatches {
		result.CurrenciesSymbol = append(result.CurrenciesSymbol, symbol)
	}

	isoRegex := regexp.MustCompile(`\b[A-Z]{3}\b`)
	isoMatches := isoRegex.FindAllString(imageRawText, -1)

	for _, iso := range isoMatches {
		result.CurrenciesISO = append(result.CurrenciesISO, iso)
	}

	return result, nil
}

func (bt *BudgetTracker) SaveExpenseCategory(userId string, category ExpenseCategoryRequest) error {
	if category.Name == "" {
		return fmt.Errorf("%w: category name is empty", appErrors.ErrInvalidInput)
	}
	if category.MaxAmount > MAX_CATEGORY_AMOUNT_LIMIT {
		return fmt.Errorf("%w: category max amount is too large, the limit is: %.2f", appErrors.ErrInvalidInput, MAX_CATEGORY_AMOUNT_LIMIT)
	}
	if len(category.Name) > MAX_CATEGORY_NAME_LENGTH {
		return fmt.Errorf("%w: category name is too long for category, the limit is: %d", appErrors.ErrInvalidInput, MAX_CATEGORY_NAME_LENGTH)
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
	if category.Name == "" {
		return fmt.Errorf("%w: category name is empty", appErrors.ErrInvalidInput)
	}
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

func (bt *BudgetTracker) GetFilteredIncomeCategories(userID string, filters *IncomeCategoryList) ([]IncomeCategoryResponse, error) {
	categoriesRaw, err := bt.storage.GetFilteredIncomeCategories(userID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get income categories: %w", err)
	}

	var categories []IncomeCategoryResponse

	for _, category := range categoriesRaw {
		var usagePercent int
		if category.TargetAmount > 0 {
			usagePercent = int((category.Amount / category.TargetAmount) * 100)
		}

		category := IncomeCategoryResponse{
			ID:           category.ID,
			Name:         category.Name,
			Amount:       category.Amount,
			TargetAmount: category.TargetAmount,
			UsagePercent: usagePercent,
			CreatedAt:    category.CreatedAt,
			UpdatedAt:    category.UpdatedAt,
			Note:         category.Note,
			CreatedBy:    category.CreatedBy,
		}
		categories = append(categories, category)
	}

	return categories, nil
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

func (bt *BudgetTracker) UpdateExpenseCategory(userId string, fields UpdateExpenseCategoryRequest) (*ExpenseCategoryResponse, error) {
	if fields.NewMaxAmount > MAX_CATEGORY_AMOUNT_LIMIT {
		return nil, fmt.Errorf("%w: category max amount is too large, the limit is: %.2f", appErrors.ErrInvalidInput, MAX_CATEGORY_AMOUNT_LIMIT)
	}
	if len(fields.NewName) > MAX_CATEGORY_NAME_LENGTH {
		return nil, fmt.Errorf("%w: category name is too long for category, the limit is: %d", appErrors.ErrInvalidInput, MAX_CATEGORY_NAME_LENGTH)
	}
	if len(fields.NewNote) > MAX_TRANSACTION_NOTE_LENGTH {
		return nil, fmt.Errorf("%w: note so long, maximum allowed length is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_NOTE_LENGTH)
	}

	fields.UpdateTime = time.Now().UTC()
	categoryRaw, err := bt.storage.UpdateExpenseCategory(userId, fields)
	if err != nil {
		return nil, fmt.Errorf("failed to update expense category: %w", err)
	}
	var usagePercent int
	if categoryRaw.MaxAmount > 0 {
		usagePercent = int((categoryRaw.Amount / categoryRaw.MaxAmount) * 100)
	}

	isExpired := time.Now().UTC().After(categoryRaw.CreatedAt.AddDate(0, 0, categoryRaw.PeriodDay))

	category := ExpenseCategoryResponse{
		ID:           categoryRaw.ID,
		Name:         categoryRaw.Name,
		Amount:       categoryRaw.Amount,
		MaxAmount:    categoryRaw.MaxAmount,
		PeriodDay:    categoryRaw.PeriodDay,
		UsagePercent: usagePercent,
		CreatedAt:    categoryRaw.CreatedAt,
		UpdatedAt:    categoryRaw.UpdatedAt,
		Note:         categoryRaw.Note,
		CreatedBy:    categoryRaw.CreatedBy,
		IsExpired:    isExpired,
	}

	return &category, nil
}

func (bt *BudgetTracker) UpdateIncomeCategory(userId string, fields UpdateIncomeCategoryRequest) (*IncomeCategoryResponse, error) {
	if fields.NewTargetAmount > MAX_TARGET_AMOUNT_LIMIT {
		return nil, fmt.Errorf("%w: category max amount is too large; the limit is: %.2f", appErrors.ErrInvalidInput, MAX_CATEGORY_AMOUNT_LIMIT)
	}
	if len(fields.NewName) > MAX_CATEGORY_NAME_LENGTH {
		return nil, fmt.Errorf("%w: category name is too long for category; the limit is: %d", appErrors.ErrInvalidInput, MAX_CATEGORY_NAME_LENGTH)
	}
	if len(fields.NewNote) > MAX_TRANSACTION_NOTE_LENGTH {
		return nil, fmt.Errorf("%w: note so long, maximum allowed length is: %d", appErrors.ErrInvalidInput, MAX_TRANSACTION_NOTE_LENGTH)
	}

	fields.UpdateTime = time.Now().UTC()
	category, err := bt.storage.UpdateIncomeCategory(userId, fields)
	if err != nil {
		return nil, fmt.Errorf("failed to update income category: %w", err)
	}

	var usagePercent int
	if category.TargetAmount > 0 {
		usagePercent = int((category.Amount / category.TargetAmount) * 100)
	}
	category.UsagePercent = usagePercent
	return category, nil
}

func (bt *BudgetTracker) DeleteIncomeCategory(userId string, categoryId string) error {
	err := bt.storage.DeleteIncomeCategory(userId, categoryId)
	if err != nil {
		return fmt.Errorf("%w:", err)
	}
	return nil
}

func (bt *BudgetTracker) DeleteExpenseCategory(userId string, categoryId string) error {
	err := bt.storage.DeleteExpenseCategory(userId, categoryId)
	if err != nil {
		return fmt.Errorf("%w:", err)
	}
	return nil
}

func (bt *BudgetTracker) GetFilteredTransactions(userID string, filters *TransactionList) ([]Transaction, error) {
	ts, err := bt.storage.GetFilteredTransactions(userID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	var transactions []Transaction
	for _, transaction := range ts {
		t := Transaction{
			ID:           transaction.ID,
			CategoryName: transaction.CategoryName,
			CategoryType: transaction.CategoryType,
			Amount:       transaction.Amount,
			Currency:     transaction.Currency,
			CreatedAt:    transaction.CreatedAt,
			Note:         transaction.Note,
			CreatedBy:    transaction.CreatedBy,
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (bt *BudgetTracker) GetTranscationById(userId string, transactionId string) (Transaction, error) {
	t, err := bt.storage.GetTransactionById(userId, transactionId)
	if err != nil {
		return Transaction{}, fmt.Errorf("failed to get transaction by id: %w", err)
	}
	return t, nil
}

func (bt *BudgetTracker) LogoutUser(userId string, token string) error {
	err := bt.storage.LogoutUser(userId, token)
	if err != nil {
		return err
	}
	return nil
}
