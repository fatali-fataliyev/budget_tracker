package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"

	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
)

// REQUESTS START:
type CreateTransactionRequest struct {
	CategoryName string  `json:"category_name"`
	CategoryType string  `json:"category_type"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Note         string  `json:"note"`
}

type SaveUserRequest struct {
	UserName string `json:"username"`
	FullName string `json:"fullname"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type DeleteUserRequest struct {
	Password string `json:"password"`
	Reason   string `json:"reason"`
}

type UserLoginRequest struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type ExpenseCategoryRequest struct {
	Name      string  `json:"name"`
	MaxAmount float64 `json:"max_amount"`
	PeriodDay int     `json:"period_day"`
	Note      string  `json:"note"`
}

type IncomeCategoryRequest struct {
	Name         string `json:"name"`
	TargetAmount int    `json:"target_amount"`
	Note         string `json:"note"`
}

type UpdateExpenseCategoryRequest struct {
	ID           string  `json:"id"`
	NewName      string  `json:"new_name"`
	NewMaxAmount float64 `json:"new_max_amount"`
	NewPeriodDay int     `json:"new_period_day"`
	NewNote      string  `json:"new_note"`
}

type UpdateIncomeCategoryRequest struct {
	ID              string `json:"id"`
	NewName         string `json:"new_name"`
	NewTargetAmount int    `json:"new_target_amount"`
	NewNote         string `json:"new_note"`
}

//REQUESTS END:

//RESPONSES:

type OperationResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Extra   string `json:"extra"`
}
type TransactionItem struct {
	ID           string  `json:"id"`
	CategoryName string  `json:"category_name"`
	CategoryType string  `json:"category_type"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	CreatedAt    string  `json:"created_at"`
	Note         string  `json:"note"`
	CreatedBy    string  `json:"created_by"`
}
type ListTransactionResponse struct {
	Transactions []TransactionItem `json:"transactions"`
}

type ExpenseCategoryResponseItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	MaxAmount    float64 `json:"max_amount"`
	PeriodDay    int     `json:"period_day"`
	IsExpired    bool    `json:"is_expired"`
	UsagePercent int     `json:"usage_percent"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	Note         string  `json:"note"`
	CreatedBy    string  `json:"created_by"`
}

type ExpenseStatsResponse struct {
	MoreThan1000      int `json:"more_than_1000"`
	Between500And1000 int `json:"between_500_and_1000"`
	LessThan500       int `json:"less_than_500"`
}

type IncomeStatsResponse struct {
	MoreThan1000      int `json:"more_than_1000"`
	Between500And1000 int `json:"between_500_and_1000"`
	LessThan500       int `json:"less_than_500"`
}

type TransactionStatsResponse struct {
	Expenses int `json:"expenses"`
	Incomes  int `json:"incomes"`
	Total    int `json:"total"`
}

type ListExpenseCategories struct {
	Categories []ExpenseCategoryResponseItem `json:"categories"`
}

type IncomeCategoryResponseItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	TargetAmount float64 `json:"target_amount"`
	UsagePercent int     `json:"usage_percent"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	Note         string  `json:"note"`
	CreatedBy    string  `json:"created_by"`
}

type ListIncomeCategories struct {
	Categories []IncomeCategoryResponseItem `json:"categories"`
}

type ImageToTransactionResponse struct {
	FullAmounts []string `json:"amounts"`
	Categories  []string `json:"categories"`
	Types       []string `json:"types"`
	CreatedDate []string `json:"created_date"`
}

type ProcessedImageResponseItem struct {
	Amounts          []float64 `json:"amounts"`
	CurrenciesISO    []string  `json:"currencies_iso"`
	CurrenciesSymbol []string  `json:"currencies_symbol"`
}

func HttpStatusFromErrorCode(errorCode string) int {
	switch errorCode {
	case appErrors.ErrNotFound:
		return 404 // not found
	case appErrors.ErrInvalidInput:
		return 400 // bad request
	case appErrors.ErrAuth:
		return 401 // unauthorized
	case appErrors.ErrAccessDenied:
		return 403 // access denied
	case appErrors.ErrConflict:
		return 409 // conflict
	default:
		return 500 //internal error
	}
}

func ExpenseStatsToHttp(stats budget.ExpenseStatsResponse) ExpenseStatsResponse {
	return ExpenseStatsResponse{
		MoreThan1000:      stats.MoreThan1000,
		Between500And1000: stats.Between500And1000,
		LessThan500:       stats.LessThan500,
	}
}

func IncomeStatsToHttp(stats budget.IncomeStatsResponse) IncomeStatsResponse {
	return IncomeStatsResponse{
		MoreThan1000:      stats.MoreThan1000,
		Between500And1000: stats.Between500And1000,
		LessThan500:       stats.LessThan500,
	}
}

func TransactionStatsToHttp(stats budget.TransactionStatsResponse) TransactionStatsResponse {
	return TransactionStatsResponse{
		Expenses: stats.Expenses,
		Incomes:  stats.Incomes,
		Total:    stats.Total,
	}
}

func TransactionToHttp(transcation budget.Transaction) TransactionItem {
	return TransactionItem{
		ID:           transcation.ID,
		CategoryName: transcation.CategoryName,
		CategoryType: transcation.CategoryType,
		Amount:       transcation.Amount,
		Currency:     transcation.Currency,
		CreatedAt:    transcation.CreatedAt.Format(time.RFC3339),
		Note:         transcation.Note,
		CreatedBy:    transcation.CreatedBy,
	}
}

func ExpenseCategoryToHttp(category budget.ExpenseCategoryResponse) ExpenseCategoryResponseItem {
	return ExpenseCategoryResponseItem{
		ID:           category.ID,
		Name:         category.Name,
		Amount:       category.Amount,
		MaxAmount:    category.MaxAmount,
		PeriodDay:    category.PeriodDay,
		IsExpired:    category.IsExpired,
		UsagePercent: category.UsagePercent,
		CreatedAt:    category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    category.UpdatedAt.Format(time.RFC3339),
		Note:         category.Note,
		CreatedBy:    category.CreatedBy,
	}
}

func IncomeCategoryToHttp(category budget.IncomeCategoryResponse) IncomeCategoryResponseItem {
	return IncomeCategoryResponseItem{
		ID:           category.ID,
		Name:         category.Name,
		Amount:       category.Amount,
		TargetAmount: category.TargetAmount,
		UsagePercent: category.UsagePercent,
		CreatedAt:    category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    category.UpdatedAt.Format(time.RFC3339),
		Note:         category.Note,
		CreatedBy:    category.CreatedBy,
	}
}

func ProcessedImageToHttp(processedImg budget.ProcessedImageResponse) ProcessedImageResponseItem {
	return ProcessedImageResponseItem{
		Amounts:          processedImg.Amounts,
		CurrenciesISO:    processedImg.CurrenciesISO,
		CurrenciesSymbol: processedImg.CurrenciesSymbol,
	}
}

func IncomeCategoryCheckParams(params url.Values) (*budget.IncomeCategoryList, error) {
	var filters budget.IncomeCategoryList

	if len(params) == 0 {
		filters.IsAllNil = true
		return &filters, nil
	}

	hasAnyFilter := false

	names := params.Get("names")
	if names != "" {
		filters.Names = strings.Split(names, ",")
		hasAnyFilter = true
	} else {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Names parameter is required!",
		}
	}

	targetAmountStr := params.Get("target_amount")
	if targetAmountStr != "" {
		targetAmount, err := strconv.ParseFloat(targetAmountStr, 64)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid target amount: %v", err.Error()),
			}
		}
		filters.TargetAmount = targetAmount
		hasAnyFilter = true
	}

	createdAtStr := params.Get("created_at")
	if createdAtStr != "" {
		createdAt, err := time.Parse("2006-01-02", createdAtStr)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid created date: %v", err.Error()),
			}
		}
		filters.CreatedAt = createdAt.UTC()
		hasAnyFilter = true
	}

	endDateStr := params.Get("end_date")
	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid end date: %v", err.Error()),
			}
		}
		filters.EndDate = endDate.UTC()
		hasAnyFilter = true

		if !filters.CreatedAt.IsZero() && endDate.Before(filters.CreatedAt) {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: "End date cannot be before created date",
			}
		}
	}

	filters.IsAllNil = !hasAnyFilter
	return &filters, nil
}

func ExpenseCategoryCheckParams(params url.Values) (*budget.ExpenseCategoryList, error) {
	var filters budget.ExpenseCategoryList

	if len(params) == 0 {
		filters.IsAllNil = true
		return &filters, nil
	}

	hasAnyFilter := false

	names := params.Get("names")
	if names != "" {
		filters.Names = strings.Split(names, ",")
		hasAnyFilter = true
	} else {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Names parameter is required!",
		}
	}

	maxAmountStr := params.Get("max_amount")
	if maxAmountStr != "" {
		maxAmount, err := strconv.ParseFloat(maxAmountStr, 64)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid maxiumum amount: %v", err.Error()),
			}
		}
		filters.MaxAmount = maxAmount
		hasAnyFilter = true
	}

	periodDayStr := params.Get("period_day")
	if periodDayStr != "" {
		periodDay, err := strconv.Atoi(periodDayStr)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid expire in day: %v", err.Error()),
			}
		}

		filters.PeriodDay = periodDay
		hasAnyFilter = true
	}

	createdAtStr := params.Get("created_at")
	if createdAtStr != "" {
		createdAt, err := time.Parse("2006-01-02", createdAtStr)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid created date: %v", err.Error()),
			}
		}
		filters.CreatedAt = createdAt.UTC()
		hasAnyFilter = true
	}

	endDateStr := params.Get("end_date")
	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid end date: %v", err.Error()),
			}
		}
		filters.EndDate = endDate.UTC()
		hasAnyFilter = true

		if !filters.CreatedAt.IsZero() && endDate.Before(filters.CreatedAt) {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: "End date cannot be before created date",
			}
		}
	}

	filters.IsAllNil = !hasAnyFilter
	return &filters, nil
}

func TransactionCheckParams(params url.Values) (*budget.TransactionList, error) {
	var filters budget.TransactionList

	if len(params) == 0 {
		filters.IsAllNil = true
		return &filters, nil
	}

	hasAnyFilter := false

	categoryNames := params.Get("category_names")

	if categoryNames != "" {
		filters.CategoryNames = strings.Split(categoryNames, ",")
		hasAnyFilter = true
	} else {
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category names parameter is required!",
		}
	}

	amount := params.Get("amount")
	if amount != "" {
		maxAmount, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid maxiumum amount: %v", err.Error()),
			}
		}

		filters.Amount = maxAmount
		hasAnyFilter = true
	}

	currency := params.Get("currency")
	if currency != "" {
		filters.Currency = currency
		hasAnyFilter = true
	}

	createdAtStr := params.Get("created_at")
	if createdAtStr != "" {
		createdAt, err := time.Parse("02/01/2006", createdAtStr)
		if err != nil {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: fmt.Sprintf("Invalid created date: %v", err.Error()),
			}
		}

		filters.CreatedAt = createdAt.UTC()
		hasAnyFilter = true
	}

	categoryType := params.Get("category_type")
	if categoryType != "" {
		if categoryType != "income" && categoryType != "expense" {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: "Invalid category type.",
			}
		} else {
			if categoryType == "income" {
				filters.Type = "+"
				hasAnyFilter = true
			} else {
				filters.Type = "-"
				hasAnyFilter = true
			}
		}

	}

	filters.IsAllNil = !hasAnyFilter
	return &filters, nil
}
