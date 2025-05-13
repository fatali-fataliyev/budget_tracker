package api

import (
	"errors"
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

//REQUESTS END:

//RESPONSES:

type UserCreatedResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}
type TransactionItem struct {
	ID           string  `json:"id"`
	Amount       float64 `json:"amount"`
	Limit        float64 `json:"limit"`
	UsagePercent int     `json:"usage_percent"`
	Currency     string  `json:"currency"`
	Category     string  `json:"category"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	Type         string  `json:"type"`
	CreatedBy    string  `json:"created_by"`
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
type ListExpenseCategories struct {
	Categories []ExpenseCategoryResponseItem `json:"categories"`
}

type InomeCategoryResponseItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	TargetAmount float64 `json:"target_amount"`
	UsagePercent int     `json:"usage_percent"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	Note         string  `json:"note"`
	CreatedBy    string  `json:"created_by"`
}

type ListIncomeCategories struct {
	Categories []InomeCategoryResponseItem `json:"categories"`
}

type ListTransactionResponse struct {
	Transactions []TransactionItem `json:"transactions"`
}

type GetTotalsResponse struct {
	Currency string  `json:"currency"`
	Type     string  `json:"type"`
	Total    float64 `json:"total"`
}

func httpStatusFromError(err error) int {
	switch {
	case errors.Is(err, appErrors.ErrNotFound):
		return 404 // not found
	case errors.Is(err, appErrors.ErrInvalidInput):
		return 400 // bad request
	case errors.Is(err, appErrors.ErrAuth):
		return 401 // unauthorized
	case errors.Is(err, appErrors.ErrAccessDenied):
		return 403 // access denied
	case errors.Is(err, appErrors.ErrConflict):
		return 409 // conflict
	default:
		return 500 //internal error
	}
}

func TransactionToHttp(transcation budget.Transaction) TransactionItem {
	return TransactionItem{
		// ID:           transcation.ID,
		// Amount:       transcation.Amount,
		// Limit:        transcation.Limit,
		// UsagePercent: transcation.UsagePercent,
		// Currency:     transcation.Currency,
		// Category:     transcation.Category,
		// CreatedAt:    transcation.CreatedDate.Format("02/01/2006 15:04"),
		// UpdatedAt:    transcation.UpdatedDate.Format("02/01/2006 15:04"),
		// Type:         transcation.Type,
		// CreatedBy:    transcation.CreatedBy,
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
		CreatedAt:    category.CreatedAt.Format("02/01/2006 15:04"),
		UpdatedAt:    category.UpdatedAt.Format("02/01/2006 15:04"),
		Note:         category.Note,
		CreatedBy:    category.CreatedBy,
	}
}

func IncomeCategoryToHttp(category budget.InomeCategoryResponse) InomeCategoryResponseItem {
	return InomeCategoryResponseItem{
		ID:           category.ID,
		Name:         category.Name,
		TargetAmount: category.TargetAmount,
		UsagePercent: category.UsagePercent,
		CreatedAt:    category.CreatedAt.Format("02/01/2006 15:04"),
		UpdatedAt:    category.UpdatedAt.Format("02/01/2006 15:04"),
		Note:         category.Note,
		CreatedBy:    category.CreatedBy,
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
	}

	targetAmount := params.Get("target_amount")
	if targetAmount != "" {
		targetAmount, err := strconv.ParseFloat(targetAmount, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid target_amount: %s", appErrors.ErrInvalidInput, targetAmount)
		}
		filters.TargetAmount = targetAmount
		hasAnyFilter = true
	}

	createdAtStr := params.Get("created_at")
	if createdAtStr != "" {
		createdAt, err := time.Parse("02/01/2006", createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid created at date: %s", appErrors.ErrInvalidInput, createdAtStr)
		}
		filters.CreatedAt = createdAt.UTC()
		hasAnyFilter = true
	}

	endDateStr := params.Get("end_date")
	if endDateStr != "" {
		endDate, err := time.Parse("02/01/2006", endDateStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid end date: %s", appErrors.ErrInvalidInput, endDateStr)
		}
		filters.EndDate = endDate.UTC()
		hasAnyFilter = true

		if !filters.CreatedAt.IsZero() && endDate.Before(filters.CreatedAt) {
			return nil, fmt.Errorf("%w: end date cannot be before created at date", appErrors.ErrInvalidInput)
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
	}

	maxAmountStr := params.Get("max_amount")
	if maxAmountStr != "" {
		maxAmount, err := strconv.ParseFloat(maxAmountStr, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid max amount: %s", appErrors.ErrInvalidInput, maxAmountStr)
		}
		filters.MaxAmount = maxAmount
		hasAnyFilter = true
	}

	periodDayStr := params.Get("period_day")
	if periodDayStr != "" {
		periodDay, err := strconv.Atoi(periodDayStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid period day: %s", appErrors.ErrInvalidInput, periodDayStr)
		}
		filters.PeriodDay = periodDay
		hasAnyFilter = true
	}

	createdAtStr := params.Get("created_at")
	if createdAtStr != "" {
		createdAt, err := time.Parse("02/01/2006", createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid created at date: %s", appErrors.ErrInvalidInput, createdAtStr)
		}
		filters.CreatedAt = createdAt.UTC()
		hasAnyFilter = true
	}

	endDateStr := params.Get("end_date")
	if endDateStr != "" {
		endDate, err := time.Parse("02/01/2006", endDateStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid end date: %s", appErrors.ErrInvalidInput, endDateStr)
		}
		filters.EndDate = endDate.UTC()
		hasAnyFilter = true

		if !filters.CreatedAt.IsZero() && endDate.Before(filters.CreatedAt) {
			return nil, fmt.Errorf("%w: end date cannot be before created at date", appErrors.ErrInvalidInput)
		}
	}

	filters.IsAllNil = !hasAnyFilter
	return &filters, nil
}
