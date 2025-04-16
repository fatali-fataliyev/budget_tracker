package api

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
)

// REQUESTS START:
type CreateTransactionRequest struct {
	Ttype    string  `json:"type"`
	Amount   float64 `json:"amount"`
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
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Category    string  `json:"category"`
	UpdatedDate string  `json:"updated_date"`
	Type        string  `json:"type"`
}

type NewUserRequest struct {
	UserName string `json:"username"`
	FullName string `json:"fullname"`
	NickName string `json:"nickname"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

//REQUESTS END:

//RESPONSES:

type UserCreatedResponse struct {
	Message string `json:"message"`
}

type AuthenticationResponse struct {
	Message string
	Token   string
}

type TransactionItem struct {
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Category    string  `json:"category"`
	CreatedDate string  `json:"created_date"`
	UpdatedDate string  `json:"updated_date"`
	Type        string  `json:"type"`
	CreatedBy   string  `json:"created_by"`
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

func ListValidateParams(params url.Values) (*budget.ListTransactionsFilters, error) {
	filters := budget.ListTransactionsFilters{}
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
