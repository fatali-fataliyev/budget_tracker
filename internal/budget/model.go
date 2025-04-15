package budget

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Transaction struct {
	ID          string
	Amount      float64
	Currency    string
	Category    string
	CreatedDate time.Time
	UpdatedDate time.Time
	Type        string
	CreatedBy   string
}

type UpdateTransactionItem struct {
	ID          string
	Amount      float64
	Currency    string
	Category    string
	UpdatedDate time.Time
	Type        string
}

type GetTotals struct {
	Type     string
	Currency string
	Total    float64
}

func (list *GetTotals) GetTotalValidate(params url.Values) (*GetTotals, error) {
	filters := GetTotals{}
	typeRaw := params.Get("type")
	currencyRaw := params.Get("currency")

	var tType string
	var currency string
	if typeRaw != "" {
		if typeRaw == "income" {
			tType = "+"
		} else if typeRaw == "expense" {
			tType = "-"
		} else {
			return nil, fmt.Errorf("invalid transaction type")
		}
	} else {
		return nil, fmt.Errorf("type is required")
	}
	if currencyRaw != "" {
		currency = currencyRaw
	} else {
		return nil, fmt.Errorf("currency is requried")
	}

	filters.Type = tType
	filters.Currency = currency

	return &filters, nil
}

type ListTransactionsFilters struct {
	Categories []string
	Type       *string
	MinAmount  *float64
	MaxAmount  *float64
	IsAllNil   bool
}

func (list *ListTransactionsFilters) ListValidateParams(params url.Values) (*ListTransactionsFilters, error) {
	filters := ListTransactionsFilters{}
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
