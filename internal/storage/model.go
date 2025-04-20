package storage

type dbTransaction struct {
	ID          string
	Amount      float64
	Limit       float64
	Currency    string
	Category    string
	CreatedDate string
	UpdatedDate string
	Type        string
	CreatedBy   string
}

type dbSession struct {
	ID        string
	Token     string
	CreatedAt string
	ExpireAt  string
	UserID    string
}
