package storage

type dbSession struct {
	ID        string
	Token     string
	CreatedAt string
	ExpireAt  string
	UserID    string
}

type dbExpenseStats struct {
	AmountRange string
	Count       int
}

type dbIncomeStats struct {
	AmountRange string
	Count       int
}

type dbTransactionStats struct {
	Expenses float64
	Incomes  float64
	Total    float64
}
