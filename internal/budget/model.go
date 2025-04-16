package budget

import (
	"fmt"
	"net/url"
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
