package api

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
