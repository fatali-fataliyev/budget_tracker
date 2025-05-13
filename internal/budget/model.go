package budget

import (
	"fmt"
	"net/url"
	"time"
)

type ExpenseCategoryRequest struct {
	Name      string
	MaxAmount float64
	PeriodDay int
	Note      string
	Type      string
}

type IncomeCategoryRequest struct {
	Name         string
	TargetAmount int
	Note         string
	Type         string
}
type ExpenseCategory struct {
	ID        string
	Name      string
	MaxAmount float64
	PeriodDay int
	CreatedAt time.Time
	UpdatedAt time.Time
	Note      string
	CreatedBy string
	Type      string
}

type ExpenseCategoryResponse struct {
	ID           string
	Name         string
	Amount       float64
	MaxAmount    float64
	PeriodDay    int
	IsExpired    bool
	UsagePercent int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Note         string
	CreatedBy    string
}

type IncomeCategory struct {
	ID           string
	Name         string
	TargetAmount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Note         string
	CreatedBy    string
	Type         string
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

type ExpenseCategoryList struct {
	Names     []string
	MaxAmount float64
	PeriodDay int
	CreatedAt time.Time
	IsAllNil  bool
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
