package budget

import (
	"time"
)

// REQUESTS START:
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

type TransactionRequest struct {
	CategoryName string
	CategoryType string
	Amount       float64
	Currency     string
	Note         string
}

type UpdateExpenseCategoryRequest struct {
	ID           string
	NewName      string
	NewMaxAmount float64
	NewPeriodDay int
	NewNote      string
	UpdateTime   time.Time
}

type UpdateIncomeCategoryRequest struct {
	ID              string
	NewName         string
	NewTargetAmount int
	NewNote         string
	UpdateTime      time.Time
}

// REQUESTS END:

// MODELS:

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

type Transaction struct {
	ID           string
	CategoryName string
	CategoryType string
	Amount       float64
	Currency     string
	CreatedAt    time.Time
	Note         string
	CreatedBy    string
}

// RESPONSES:
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

type ExpenseStatsResponse struct {
	MoreThan1000      int
	Between500And1000 int
	LessThan500       int
}

type IncomeStatsResponse struct {
	MoreThan1000      int
	Between500And1000 int
	LessThan500       int
}

type IncomeCategoryResponse struct {
	ID           string
	Name         string
	Amount       float64
	TargetAmount float64
	UsagePercent int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Note         string
	CreatedBy    string
}

type IncomeCategoryList struct {
	Names        []string
	TargetAmount float64
	CreatedAt    time.Time
	EndDate      time.Time
	IsAllNil     bool
}

type ExpenseCategoryList struct {
	Names     []string
	MaxAmount float64
	PeriodDay int
	CreatedAt time.Time
	EndDate   time.Time
	IsAllNil  bool
}

type TransactionList struct {
	CategoryNames []string
	Amount        float64
	Currency      string
	CreatedAt     time.Time
	Type          string
	IsAllNil      bool
}

type ProcessedImageResponse struct {
	Amounts          []float64
	CurrenciesISO    []string
	CurrenciesSymbol []string
}
