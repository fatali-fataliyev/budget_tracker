package api

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
)

// REQUESTS START:
type CreateTransactionRequest struct {
	Ttype    string  `json:"type"`
	Amount   float64 `json:"amount"`
	Limit    float64 `json:"limit"`
	Category string  `json:"category"`
	Currency string  `json:"currency"`
}

type CreateUserRequest struct {
	UserName string `json:"username"`
	FullName string `json:"fullname"`
	NickName string `json:"nickname"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLoginRequest struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}
type UpdateTransactionRequest struct {
	Type     string  `json:"type"`
	Amount   float64 `json:"amount"`
	Limit    float64 `json:"limit"`
	Category string  `json:"category"`
	Currency string  `json:"currency"`
}

type NewUserRequest struct {
	UserName string `json:"username"`
	FullName string `json:"fullname"`
	NickName string `json:"nickname"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type NewCategoryRequest struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	MaxAmount  float64 `json:"max_amount"`
	PeriodDays int     `json:"period_days"`
}

//REQUESTS END:

//RESPONSES:

type UserCreatedResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type AuthenticationResponse struct {
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

type CategoryItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	CreatedDate  string  `json:"created_at"`
	UpdatedDate  string  `json:"updated_at"`
	MaxAmount    float64 `json:"limit"`
	PeriodDays   int     `json:"period_days"`
	UsagePercent int     `json:"usage_percent"`
	CreatedBy    string  `json:"string"`
}

type ListTransactionResponse struct {
	Transactions []TransactionItem `json:"transactions"`
}

type ListCategoriesResponse struct {
	Categories []CategoryItem `json:"categories"`
}

type GetTotalsResponse struct {
	Currency string  `json:"currency"`
	Type     string  `json:"type"`
	Total    float64 `json:"total"`
}

func httpStatusFromError(err error) int {
	switch {
	case errors.Is(err, budget.ErrNotFound):
		return 404 // not found
	case errors.Is(err, budget.ErrInvalidInput):
		return 400 // bad request
	case errors.Is(err, budget.ErrAuth):
		return 401 // unauthorized
	case errors.Is(err, budget.ErrAccessDenied):
		return 403 // access denied
	case errors.Is(err, budget.ErrConflict):
		return 409 // conflict
	default:
		return 500 //internal error
	}
}

func TransactionToHttp(transcation budget.Transaction) TransactionItem {
	return TransactionItem{
		ID:           transcation.ID,
		Amount:       transcation.Amount,
		Limit:        transcation.Limit,
		UsagePercent: transcation.UsagePercent,
		Currency:     transcation.Currency,
		Category:     transcation.Category,
		CreatedAt:    transcation.CreatedDate.Format("02/01/2006 15:04"),
		UpdatedAt:    transcation.UpdatedDate.Format("02/01/2006 15:04"),
		Type:         transcation.Type,
		CreatedBy:    transcation.CreatedBy,
	}
}

func CategoryToHttp(category budget.Category) CategoryItem {
	return CategoryItem{
		ID:           category.ID,
		Name:         category.Name,
		Type:         category.Type,
		CreatedDate:  category.CreatedDate.Format("02/01/2006 15:04"),
		UpdatedDate:  category.UpdatedDate.Format("02/01/2006 15:04"),
		MaxAmount:    category.MaxAmount,
		PeriodDays:   category.PeriodDays,
		UsagePercent: category.UsagePercent,
		CreatedBy:    category.CreatedBy,
	}
}

func ListValidateParams(params url.Values) (*budget.ListTransactionsFilters, error) {
	var filters budget.ListTransactionsFilters
	if len(params) == 0 {
		filters.IsAllNil = true
		return &filters, nil
	}

	var categoriesWithEmptyStrings []string
	var categories []string
	categoryRaw := params.Get("categories")
	categoriesWithEmptyStrings = strings.Split(categoryRaw, ",")

	for _, category := range categoriesWithEmptyStrings {
		trimmed := strings.TrimSpace(category)
		if trimmed != "" {
			categories = append(categories, trimmed)
		}
	}

	transactionType := params.Get("type")
	minAmount := params.Get("min")
	maxAmount := params.Get("max")

	var maxAmountFloat *float64
	var minAmountFloat *float64

	if minAmount != "" {
		parsedMinAmount, err := strconv.ParseFloat(minAmount, 64)
		if err == nil {
			minAmountFloat = &parsedMinAmount
		}
		if err != nil {
			return nil, fmt.Errorf("invalid minimum amount")
		}
	} else {
		minAmountFloat = nil
	}
	if maxAmount != "" {
		parsedMaxAmount, err := strconv.ParseFloat(maxAmount, 64)
		if err == nil {
			maxAmountFloat = &parsedMaxAmount
		}
		if err != nil {
			return nil, fmt.Errorf("invalid maximum amount")
		}
	} else {
		minAmountFloat = nil
	}
	if transactionType != "" {
		if transactionType == "income" {
			income := "+"
			filters.Type = &income
		} else if transactionType == "expense" {
			expense := "-"
			filters.Type = &expense
		} else {
			return nil, fmt.Errorf("invalid transaction type")
		}
	}

	filters.MinAmount = minAmountFloat
	filters.MaxAmount = maxAmountFloat
	filters.Categories = categories
	return &filters, nil
}

func CategoriesListValidateParams(params url.Values) (*budget.CategoriesListFilters, error) {
	var filters budget.CategoriesListFilters
	if len(params) == 0 {
		filters.IsAllNil = true
		return &filters, nil
	}

	categoryType := strings.ToLower(params.Get("type"))
	if categoryType != "income" && categoryType != "expense" {
		return nil, fmt.Errorf("%w: invalid category type: %s", budget.ErrInvalidInput, categoryType)
	} else {
		if categoryType == "income" {
			filters.Type = "+"
		} else {
			categoryType = "-"
		}
	}

	if periodStr := params.Get("period"); periodStr != "" {
		if periodInt, err := strconv.Atoi(periodStr); err == nil {
			filters.PeriodDays = periodInt
		} else {
			return nil, fmt.Errorf("failed to convert string periods days to integer: %w", err)
		}
	}

	if namesStr := params.Get("names"); namesStr != "" {
		filters.Names = strings.Split(namesStr, ",")
	}

	if limitStr := params.Get("limit"); limitStr != "" {
		if limitInt, err := strconv.Atoi(limitStr); err == nil {
			filters.LimitAmount = float64(limitInt)
		} else {
			return nil, fmt.Errorf("failed to convert string category limit to integer: %w", err)
		}
	}

	layout := "2006-01-02" // il ay gun
	if start := params.Get("startDate"); start != "" {
		if date, err := time.Parse(layout, start); err == nil {
			filters.StartDate = date
		} else {
			return nil, fmt.Errorf("failed to convert start date: %w", err)
		}
	}
	if end := params.Get("endDate"); end != "" {
		if date, err := time.Parse(layout, end); err == nil {
			filters.EndDate = date
		} else {
			return nil, fmt.Errorf("failed to convert end date: %w", err)
		}
	}

	return &filters, nil
}
