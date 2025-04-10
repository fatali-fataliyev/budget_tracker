package budget

import (
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
