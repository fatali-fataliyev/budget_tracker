package budget

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	appErrors "github.com/fatali-fataliyev/budget_tracker/customErrors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/contextutil"
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
	SaveUser(ctx context.Context, newUser auth.User) error
	SaveSession(ctx context.Context, session auth.Session) error
	SaveExpenseCategory(ctx context.Context, category ExpenseCategory) error
	SaveIncomeCategory(ctx context.Context, category IncomeCategory) error
	CheckSession(ctx context.Context, token string) (userId string, err error)
	UpdateSession(ctx context.Context, userId string, expireAt time.Time) error
	GetSessionByToken(ctx context.Context, token string) (auth.Session, error)
	SaveTransaction(ctx context.Context, t Transaction) error
	GetFilteredTransactions(ctx context.Context, userID string, filters *TransactionList) ([]Transaction, error)
	GetFilteredExpenseCategories(ctx context.Context, userID string, filters *ExpenseCategoryList) ([]ExpenseCategoryResponse, error)
	GetFilteredIncomeCategories(ctx context.Context, userID string, filters *IncomeCategoryList) ([]IncomeCategoryResponse, error)
	GetTransactionById(ctx context.Context, userID string, transacationID string) (Transaction, error)
	GetExpenseCategoryStats(ctx context.Context, userId string) (ExpenseStatsResponse, error)
	GetIncomeCategoryStats(ctx context.Context, userId string) (IncomeStatsResponse, error)
	GetTransactionStats(ctx context.Context, userId string) (TransactionStatsResponse, error)
	ValidateUser(ctx context.Context, credentials auth.UserCredentialsPure) (auth.User, error)
	IsUserExists(ctx context.Context, username string) (bool, error)
	IsEmailConfirmed(ctx context.Context, emailAddress string) (bool, error)
	UpdateExpenseCategory(ctx context.Context, userId string, fields UpdateExpenseCategoryRequest) (*ExpenseCategoryResponse, error)
	DeleteExpenseCategory(ctx context.Context, userId string, categoryId string) error
	DeleteIncomeCategory(ctx context.Context, userId string, categoryId string) error
	UpdateIncomeCategory(ctx context.Context, userId string, fields UpdateIncomeCategoryRequest) (*IncomeCategoryResponse, error)
	LogoutUser(ctx context.Context, userId string, token string) error
	DeleteUser(ctx context.Context, userId string, deleteReq auth.DeleteUser) error
	GetUserData(ctx context.Context, userId string) (UserDataResponse, error)
	GetAccountInfo(ctx context.Context, userId string) (AccountInfo, error)
	GetStorageType() string
}

func (bt *BudgetTracker) ValidateUser(ctx context.Context, credentials auth.UserCredentialsPure) (auth.User, error) {
	if credentials.UserName == "" {
		return auth.User{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username cannot be empty!",
		}
	}
	if credentials.PasswordPlain == "" {
		return auth.User{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Password cannot be empty!",
		}
	}

	user, err := bt.storage.ValidateUser(ctx, credentials)
	if err != nil {
		return auth.User{}, err
	}
	return user, nil
}

func (bt *BudgetTracker) GenerateSession(ctx context.Context, credentialsPure auth.UserCredentialsPure) (string, error) {
	user, err := bt.storage.ValidateUser(ctx, credentialsPure)
	if err != nil {
		return "", err
	}

	tokenByte := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, tokenByte); err != nil {
		return "", err
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

	err = bt.storage.SaveSession(ctx, session)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (bt *BudgetTracker) CheckSession(ctx context.Context, token string) (string, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	session, err := bt.storage.GetSessionByToken(ctx, token)
	if err != nil {
		return "", err
	}

	logging.Logger.Infof("[TraceID=%s] | Service.CheckSession() Checking exists session", traceID)
	userId, err := bt.storage.CheckSession(ctx, token)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	daysUntilExpiry := int(session.ExpireAt.Sub(now).Hours() / 24)

	if daysUntilExpiry <= 5 {
		newExpireAt := time.Now().AddDate(0, 1, 0)
		logging.Logger.Infof("[TraceID=%s] | Service.CheckSession() calling storage.UpdateSession", traceID)
		err := bt.storage.UpdateSession(ctx, userId, newExpireAt)
		if err != nil {
			return "", err
		}
		return userId, nil
	}

	return userId, nil
}

func (bt *BudgetTracker) IsUserExists(ctx context.Context, username string) (bool, error) {
	if username == "" {
		return false, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username cannot be empty",
		}
	}

	traceID := contextutil.TraceIDFromContext(ctx)
	logging.Logger.Infof("[TraceID=%s] | Service.IsUserExists calling for username=%s to storage layer", traceID, username)
	result, err := bt.storage.IsUserExists(ctx, username)
	if err != nil {
		return false, err
	}
	return result, nil
}

func (bt *BudgetTracker) SaveUser(ctx context.Context, newUser auth.NewUser) (string, error) {
	if newUser.UserName == "" {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username cannot be empty!",
		}
	}
	if newUser.PasswordPlain == "" {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Password cannot be empty!",
		}
	}
	if newUser.Email == "" {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Password cannot be empty!",
		}
	}

	isUserExists, err := bt.IsUserExists(ctx, newUser.UserName)
	if err != nil {
		return "", err
	}
	if isUserExists {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username already taken!",
		}
	}
	isEmailTaken, err := bt.storage.IsEmailConfirmed(ctx, newUser.Email)
	traceID := contextutil.TraceIDFromContext(ctx)
	logging.Logger.Infof("[TraceID=%s] | Service calling IsEmailConfirmed from storage layer", traceID)
	if err != nil {
		return "", err
	}
	if isEmailTaken {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Email already taken!",
		}
	}
	hashedPassword, err := auth.HashPassword(ctx, newUser.PasswordPlain)
	if err != nil {
		return "", err
	}

	user := auth.User{
		ID:             uuid.New().String(),
		UserName:       strings.ToLower(newUser.UserName),
		FullName:       CapitalizeFullName(newUser.FullName),
		Email:          strings.ToLower(newUser.Email),
		PasswordHashed: hashedPassword,
		PendingEmail:   strings.ToLower(newUser.Email),
	}

	if err := bt.storage.SaveUser(ctx, user); err != nil {
		return "", err
	}

	credentials := auth.UserCredentialsPure{
		UserName:      newUser.UserName,
		PasswordPlain: newUser.PasswordPlain,
	}

	token, err := bt.GenerateSession(ctx, credentials)
	if err != nil {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Registration completed but something went wrong, try log in please.",
		}
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

func (bt *BudgetTracker) SaveTransaction(ctx context.Context, userId string, transaction TransactionRequest) error {
	if transaction.CategoryName == "" {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category name cannot be empty!",
		}
	}
	if IsFloatZero(transaction.Amount) {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Transaction amount is zero or very close to zero, please enter valid transaction amount.",
		}
	}
	if transaction.Amount > MAX_TRANSACTION_AMOUNT_LIMIT {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Maximum allowed amount per transaction is %d", MAX_TRANSACTION_AMOUNT_LIMIT),
		}
	}
	if len(transaction.CategoryName) > MAX_TRANSACTION_CATEGORY_NAME_LENGTH {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category name so long, maxiumum allowed cateogory name length %d", MAX_TRANSACTION_CATEGORY_NAME_LENGTH),
		}
	}
	if len(transaction.Currency) > MAX_TRANSACTION_CURRENCY_LENGTH {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Currency so long, maximum allowed currency length is %d", MAX_TRANSACTION_CURRENCY_LENGTH),
		}
	}
	if len(transaction.Note) > MAX_TRANSACTION_NOTE_LENGTH {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Note so long, maximum allowed note length is %d", MAX_TRANSACTION_NOTE_LENGTH),
		}
	}

	now := time.Now().UTC()
	txn := Transaction{
		ID:           uuid.New().String(),
		CategoryName: strings.ToLower(transaction.CategoryName),
		CategoryType: transaction.CategoryType,
		Amount:       transaction.Amount,
		Currency:     transaction.Currency,
		CreatedAt:    now,
		Note:         transaction.Note,
		CreatedBy:    userId,
	}

	if err := bt.storage.SaveTransaction(ctx, txn); err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) ProcessImage(ctx context.Context, imageRawText string) (ProcessedImageResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	if imageRawText == "" {
		return ProcessedImageResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Image raw text is empty.",
		}
	}

	var result ProcessedImageResponse

	amountRegex := regexp.MustCompile(`\d+(\.\d+)?`)

	amountMatches := amountRegex.FindAllString(imageRawText, -1)

	for _, amount := range amountMatches {
		num, err := strconv.ParseFloat(amount, 64)
		if err == nil {
			result.Amounts = append(result.Amounts, num)
		}
		logging.Logger.Warnf("[TraceID=%s] | failed to convert string number to float64 from Service.ProcessImage() function, Error: %v", traceID, err)
	}

	isoRegex := regexp.MustCompile(`\b[A-Z]{3}\b`)
	isoMatches := isoRegex.FindAllString(imageRawText, -1)

	for _, iso := range isoMatches {
		result.CurrenciesISO = append(result.CurrenciesISO, iso)
	}

	return result, nil
}

func (bt *BudgetTracker) SaveExpenseCategory(ctx context.Context, userId string, category ExpenseCategoryRequest) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	if category.Name == "" {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category cannot be empty!",
		}
	}
	if category.MaxAmount > MAX_CATEGORY_AMOUNT_LIMIT {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category maximum amount is too large, allowed maximum amount is %2.f", MAX_CATEGORY_AMOUNT_LIMIT),
		}
	}
	if len(category.Name) > MAX_CATEGORY_NAME_LENGTH {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category name so long, allowed maximum length is %d", MAX_CATEGORY_NAME_LENGTH),
		}
	}

	now := time.Now().UTC()
	categoryItem := ExpenseCategory{
		ID:        uuid.New().String(),
		Name:      strings.ToLower(category.Name),
		MaxAmount: category.MaxAmount,
		PeriodDay: category.PeriodDay,
		CreatedAt: now,
		UpdatedAt: now,
		Note:      category.Note,
		CreatedBy: userId,
		Type:      category.Type,
	}

	if err := bt.storage.SaveExpenseCategory(ctx, categoryItem); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | storage.SaveExpenseCateogory() failed", traceID)
		return err
	}

	return nil
}

func (bt *BudgetTracker) SaveIncomeCategory(ctx context.Context, userId string, category IncomeCategoryRequest) error {
	if category.TargetAmount > MAX_TARGET_AMOUNT_LIMIT {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category maximum amount is too large, allowed maximum amount is %2.f", MAX_CATEGORY_AMOUNT_LIMIT),
		}
	}

	if len(category.Name) > MAX_CATEGORY_NAME_LENGTH {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category name so long, allowed maximum length is %d", MAX_CATEGORY_NAME_LENGTH),
		}
	}

	now := time.Now().UTC()
	categoryItem := IncomeCategory{
		ID:           uuid.New().String(),
		Name:         strings.ToLower(category.Name),
		TargetAmount: category.TargetAmount,
		CreatedAt:    now,
		UpdatedAt:    now,
		Note:         category.Note,
		CreatedBy:    userId,
		Type:         category.Type,
	}

	if err := bt.storage.SaveIncomeCategory(ctx, categoryItem); err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) GetExpenseCategoryStats(ctx context.Context, userId string) (ExpenseStatsResponse, error) {
	stats, err := bt.storage.GetExpenseCategoryStats(ctx, userId)
	if err != nil {
		return ExpenseStatsResponse{}, err
	}

	return stats, nil
}

func (bt *BudgetTracker) GetIncomeCategoryStats(ctx context.Context, userId string) (IncomeStatsResponse, error) {
	stats, err := bt.storage.GetIncomeCategoryStats(ctx, userId)
	if err != nil {
		return IncomeStatsResponse{}, err
	}

	return stats, nil
}

func (bt *BudgetTracker) GetTransactionStats(ctx context.Context, userId string) (TransactionStatsResponse, error) {
	stats, err := bt.storage.GetTransactionStats(ctx, userId)
	if err != nil {
		return TransactionStatsResponse{}, err
	}

	return stats, nil
}

func (bt *BudgetTracker) GetFilteredIncomeCategories(ctx context.Context, userID string, filters *IncomeCategoryList) ([]IncomeCategoryResponse, error) {
	categoriesRaw, err := bt.storage.GetFilteredIncomeCategories(ctx, userID, filters)
	if err != nil {
		return nil, err
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

func (bt *BudgetTracker) GetFilteredExpenseCategories(ctx context.Context, userID string, filters *ExpenseCategoryList) ([]ExpenseCategoryResponse, error) {
	categoriesRaw, err := bt.storage.GetFilteredExpenseCategories(ctx, userID, filters)
	if err != nil {
		return nil, err
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

func (bt *BudgetTracker) UpdateExpenseCategory(ctx context.Context, userId string, fields UpdateExpenseCategoryRequest) (*ExpenseCategoryResponse, error) {
	if fields.NewMaxAmount > MAX_CATEGORY_AMOUNT_LIMIT {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category new maximum is too larger, allowed maximum length is %d", MAX_CATEGORY_NAME_LENGTH),
		}
	}
	if len(fields.NewName) > MAX_CATEGORY_NAME_LENGTH {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category new name so long, allowed maximum length is %d", MAX_CATEGORY_NAME_LENGTH),
		}
	}
	if len(fields.NewNote) > MAX_TRANSACTION_NOTE_LENGTH {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category new note so long, allowed maximum length is %d", MAX_TRANSACTION_NOTE_LENGTH),
		}
	}

	fields.UpdateTime = time.Now().UTC()
	categoryRaw, err := bt.storage.UpdateExpenseCategory(ctx, userId, fields)
	if err != nil {
		return nil, err
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

func (bt *BudgetTracker) UpdateIncomeCategory(ctx context.Context, userId string, fields UpdateIncomeCategoryRequest) (*IncomeCategoryResponse, error) {
	if fields.NewTargetAmount > MAX_TARGET_AMOUNT_LIMIT {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category new target amount is too larger, allowed maximum length is %d", MAX_CATEGORY_NAME_LENGTH),
		}
	}
	if len(fields.NewName) > MAX_CATEGORY_NAME_LENGTH {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category new name so long, allowed maximum length is %d", MAX_CATEGORY_NAME_LENGTH),
		}
	}
	if len(fields.NewNote) > MAX_TRANSACTION_NOTE_LENGTH {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Category new note so long, allowed maximum length is %d", MAX_TRANSACTION_NOTE_LENGTH),
		}
	}

	fields.UpdateTime = time.Now().UTC()
	category, err := bt.storage.UpdateIncomeCategory(ctx, userId, fields)
	if err != nil {
		return nil, err
	}

	var usagePercent int
	if category.TargetAmount > 0 {
		usagePercent = int((category.Amount / category.TargetAmount) * 100)
	}
	category.UsagePercent = usagePercent
	return category, nil
}

func (bt *BudgetTracker) DeleteIncomeCategory(ctx context.Context, userId string, categoryId string) error {
	err := bt.storage.DeleteIncomeCategory(ctx, userId, categoryId)
	if err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) DeleteExpenseCategory(ctx context.Context, userId string, categoryId string) error {
	err := bt.storage.DeleteExpenseCategory(ctx, userId, categoryId)
	if err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) GetFilteredTransactions(ctx context.Context, userID string, filters *TransactionList) ([]Transaction, error) {
	ts, err := bt.storage.GetFilteredTransactions(ctx, userID, filters)
	if err != nil {
		return nil, err
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

func (bt *BudgetTracker) GetTranscationById(ctx context.Context, userId string, transactionId string) (Transaction, error) {
	t, err := bt.storage.GetTransactionById(ctx, userId, transactionId)
	if err != nil {
		return Transaction{}, err
	}
	return t, nil
}

func (bt *BudgetTracker) LogoutUser(ctx context.Context, userId string, token string) error {
	err := bt.storage.LogoutUser(ctx, userId, token)
	if err != nil {
		return err
	}
	return nil
}

func (bt *BudgetTracker) DownloadUserData(ctx context.Context, userId string) (UserDataResponse, error) {
	data, err := bt.storage.GetUserData(ctx, userId)
	if err != nil {
		return UserDataResponse{}, err
	}
	return data, nil
}

func (bt *BudgetTracker) DeleteUser(ctx context.Context, userId string, deleteReq auth.DeleteUser) error {
	if deleteReq.Password == "" {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Password cannot be empty!",
		}
	}

	err := bt.storage.DeleteUser(ctx, userId, deleteReq)
	if err != nil {
		return err
	}

	return nil
}

func (bt *BudgetTracker) GetAccountInfo(ctx context.Context, userId string) (AccountInfo, error) {
	accInfo, err := bt.storage.GetAccountInfo(ctx, userId)
	if err != nil {
		return AccountInfo{}, err
	}
	return accInfo, nil
}
