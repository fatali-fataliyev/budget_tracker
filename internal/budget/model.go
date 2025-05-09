package budget

import (
	"fmt"
	"net/url"
	"time"
)

type Category struct {
	ID           string
	Name         string
	Type         string
	CreatedDate  time.Time
	UpdatedDate  time.Time
	MaxAmount    float64
	PeriodDays   int
	UsagePercent int
	CreatedBy    string
}

type TransactionRequest struct {
	Category string
	Amount   float64
	Currency string
	Note     string
}

type Transaction struct {
	ID        string
	Category  string
	Amount    float64
	Currency  string
	CreatedAt time.Time
	Note      string
	CreatedBy string
}

type ListTransactionsFilters struct {
	Categories []string
	Type       *string
	MinAmount  *float64
	MaxAmount  *float64
	IsAllNil   bool
}

// myUrl.com/category?type=income&period=7&names=food,fun&limit=500&startDate=20/09/2004&endDate=30/10/2020
//https://myUrl.com/category?type=income&period=7&names=food%2Cfun&max=500&startDate=20%2F09%2F2004&endDate=30%2F10%2F2020

type CategoriesListFilters struct {
	Type        string
	PeriodDays  int
	Names       []string
	LimitAmount float64
	StartDate   time.Time
	EndDate     time.Time
	IsAllNil    bool
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
