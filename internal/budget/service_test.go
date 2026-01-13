package budget

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	appErrors "github.com/fatali-fataliyev/budget_tracker/customErrors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
)

// Mocks
type MockStorage struct {
}

func (m *MockStorage) SaveUser(ctx context.Context, newUser auth.User) error {
	return nil
}

func (m *MockStorage) ValidateUser(ctx context.Context, creds auth.UserCredentialsPure) (auth.User, error) {
	if creds.UserName == "john" {
		return auth.User{ID: "1234", UserName: "valid_user"}, nil
	}
	return auth.User{}, errors.New("storage error")
}

func (m *MockStorage) CheckSession(ctx context.Context, token string) (userId string, err error) {
	if token == "session123" {
		return "123", nil
	}
	return "", fmt.Errorf("user does not exist")
}

func (m *MockStorage) SaveSession(ctx context.Context, session auth.Session) error {
	return nil

}

func (m *MockStorage) SaveExpenseCategory(ctx context.Context, category ExpenseCategory) error {
	return nil
}

func (m *MockStorage) SaveIncomeCategory(ctx context.Context, category IncomeCategory) error {
	return nil
}

func (m *MockStorage) UpdateSession(ctx context.Context, userId string, expireAt time.Time) error {
	return nil
}

func (m *MockStorage) GetSessionByToken(ctx context.Context, token string) (auth.Session, error) {
	if token == "tok-expired" {
		return auth.Session{
			ID:        "session-exp",
			Token:     "tok-expired",
			CreatedAt: time.Now().Add(-2 * time.Hour),
			ExpireAt:  time.Now().Add(-1 * time.Hour),
			UserID:    "john-1234",
		}, nil
	}

	return auth.Session{
		ID:        "session-valid",
		Token:     "tok-valid",
		CreatedAt: time.Now(),
		ExpireAt:  time.Now().Add(24 * time.Hour),
		UserID:    "john-1234",
	}, nil
}

func (m *MockStorage) SaveTransaction(ctx context.Context, t Transaction) error {
	return nil
}

func (m *MockStorage) GetFilteredTransactions(ctx context.Context, userID string, filters *TransactionList) ([]Transaction, error) {
	transactions := []Transaction{
		{
			ID:           "ts-1",
			CategoryName: "salary",
			CategoryType: "+",
			Amount:       30.45,
			Currency:     "USD",
			CreatedAt:    time.Now(),
			Note:         "Freelance",
			CreatedBy:    "john-1234",
		},
	}

	return transactions, nil
}

func (m *MockStorage) GetFilteredExpenseCategories(ctx context.Context, userID string, filters *ExpenseCategoryList) ([]ExpenseCategoryResponse, error) {
	categories := []ExpenseCategoryResponse{
		{
			ID:           "ts-1",
			Name:         "home repair",
			IsExpired:    false,
			UsagePercent: 40,
			Amount:       30.45,
			MaxAmount:    3000,
			PeriodDay:    7,
			CreatedAt:    time.Now(),
			Note:         "Freelance",
			CreatedBy:    "john-1234",
		},
	}

	return categories, nil
}

func (m *MockStorage) GetFilteredIncomeCategories(ctx context.Context, userID string, filters *IncomeCategoryList) ([]IncomeCategoryResponse, error) {
	categories := []IncomeCategoryResponse{
		{
			ID:           "ts-1",
			Name:         "home repair",
			UsagePercent: 40,
			Amount:       30.45,
			TargetAmount: 3000,
			CreatedAt:    time.Now(),
			Note:         "Freelance",
			CreatedBy:    "john-1234",
		},
	}

	return categories, nil
}

func (m *MockStorage) GetTransactionById(ctx context.Context, userID string, transacationID string) (Transaction, error) {
	transaction := Transaction{
		ID:           "ts-1",
		CategoryName: "salary",
		CategoryType: "+",
		Amount:       1500,
		Currency:     "USD",
		CreatedAt:    time.Now(),
		Note:         "Salary",
		CreatedBy:    "john-1234",
	}

	return transaction, nil
}

func (m *MockStorage) GetExpenseCategoryStats(ctx context.Context, userId string) (ExpenseStatsResponse, error) {
	stats := ExpenseStatsResponse{
		MoreThan1000:      30,
		Between500And1000: 20,
		LessThan500:       50,
	}

	return stats, nil
}

func (m *MockStorage) GetIncomeCategoryStats(ctx context.Context, userId string) (IncomeStatsResponse, error) {
	stats := IncomeStatsResponse{
		MoreThan1000:      30,
		Between500And1000: 20,
		LessThan500:       50,
	}

	return stats, nil
}

func (m *MockStorage) GetTransactionStats(ctx context.Context, userId string) (TransactionStatsResponse, error) {
	stats := TransactionStatsResponse{
		Expenses: 10,
		Incomes:  15,
		Total:    25,
	}

	return stats, nil
}

func (m *MockStorage) IsUserExists(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (m *MockStorage) IsEmailConfirmed(ctx context.Context, emailAddress string) (bool, error) {
	return false, nil
}

func (m *MockStorage) UpdateExpenseCategory(ctx context.Context, userId string, fields UpdateExpenseCategoryRequest) (*ExpenseCategoryResponse, error) {
	updatedExpenseCategory := ExpenseCategoryResponse{
		ID:           "ts-1",
		Name:         "home repair",
		Amount:       30.45,
		MaxAmount:    3000,
		PeriodDay:    7,
		IsExpired:    false,
		UsagePercent: 40,
		CreatedAt:    time.Now().Add(-3),
		UpdatedAt:    time.Now(),
		Note:         "Freelance",
		CreatedBy:    "john-1234",
	}

	return &updatedExpenseCategory, nil
}

func (m *MockStorage) DeleteExpenseCategory(ctx context.Context, userId string, categoryId string) error {
	return nil
}

func (m *MockStorage) DeleteIncomeCategory(ctx context.Context, userId string, categoryId string) error {
	return nil
}

func (m *MockStorage) UpdateIncomeCategory(ctx context.Context, userId string, fields UpdateIncomeCategoryRequest) (*IncomeCategoryResponse, error) {
	updatedIncomeCategory := IncomeCategoryResponse{
		ID:           "ts-1",
		Name:         "home repair",
		Amount:       30.45,
		UsagePercent: 40,
		CreatedAt:    time.Now().Add(-3),
		UpdatedAt:    time.Now(),
		Note:         "Freelance",
		CreatedBy:    "john-1234",
	}

	return &updatedIncomeCategory, nil
}

func (m *MockStorage) LogoutUser(ctx context.Context, userId string, token string) error {
	return nil
}

func (m *MockStorage) DeleteUser(ctx context.Context, userId string, deleteReq auth.DeleteUser) error {
	return nil
}

func (m *MockStorage) GetUserData(ctx context.Context, userId string) (UserDataResponse, error) {
	userData := UserDataResponse{
		Transactions:      nil,
		ExpenseCategories: nil,
		IncomeCategories:  nil,
	}

	return userData, nil
}

func (m *MockStorage) GetAccountInfo(ctx context.Context, userId string) (AccountInfo, error) {
	accountInfo := AccountInfo{
		Username: "john",
		Fullname: "John Doe",
		Email:    "john@gmail.com",
		JoinedAt: "2026-01-13",
	}

	return accountInfo, nil
}

func (m *MockStorage) GetStorageType() string {
	return "MySQL"
}

// Tests

func TestSaveUser(t *testing.T) {
	// 1. Setup the BudgetTracker with a Mock Storage
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()

	// 2. Define the "Table"
	tests := []struct {
		name        string       // Name of the test case
		input       auth.NewUser // What we send to the function
		wantToken   bool         // Do we expect a token back?
		expectedMsg string       // What error message do we expect?
	}{
		{
			name:        "Fail - Empty Username",
			input:       auth.NewUser{UserName: "", PasswordPlain: "123", Email: "john@gmail.com"},
			expectedMsg: "Username cannot be empty!",
		},
		{
			name:        "Fail - Empty Email",
			input:       auth.NewUser{UserName: "bob", PasswordPlain: "123", Email: ""},
			expectedMsg: "Email cannot be empty!",
		},
		{
			name: "Success - Valid Registration",
			input: auth.NewUser{
				UserName:      "johndoe",
				PasswordPlain: "secure123",
				Email:         "john@example.com",
				FullName:      "john doe",
			},
			expectedMsg: "Registration completed but something went wrong, try log in please.",
		},
	}

	// 3. Iterate through the table
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bt.SaveUser(ctx, tt.input)

			// Assert specific error message
			if appErr, ok := err.(appErrors.ErrorResponse); ok {
				if appErr.Message != tt.expectedMsg {
					t.Errorf("Got message %q, want %q", appErr.Message, tt.expectedMsg)
				}
			}

		})
	}
}

func TestValidateUser(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()

	tests := []struct {
		name        string
		input       auth.UserCredentialsPure
		wantUser    bool
		expectedMsg string
	}{
		{
			name:        "Fail - Empty username",
			input:       auth.UserCredentialsPure{UserName: "", PasswordPlain: "1234"},
			wantUser:    false,
			expectedMsg: "Username cannot be empty!",
		},
		{
			name:        "Fail - Empty password",
			input:       auth.UserCredentialsPure{UserName: "john", PasswordPlain: ""},
			wantUser:    false,
			expectedMsg: "Password cannot be empty!",
		},
		{
			name:     "Success - Valid user",
			input:    auth.UserCredentialsPure{UserName: "john", PasswordPlain: "john123"},
			wantUser: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := bt.ValidateUser(ctx, tt.input)

			if appErr, ok := err.(appErrors.ErrorResponse); ok {

				if tt.name == "Success - Valid user" {
					if user.ID != "1234" {
						t.Errorf("Expected valid user ID, got: %q", user.ID)
					}
				}

				if appErr.Message != tt.expectedMsg {
					t.Errorf("Got message %q, want: %q", appErr.Message, tt.expectedMsg)
				}
			}
		})
	}

}

func TestCheckSession(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  error
	}{
		{
			name:     "expired session",
			input:    "expSession",
			expected: "",
			wantErr:  fmt.Errorf("does not exist"),
		},
		{
			name:     "invalid session",
			input:    "session123",
			expected: "123",
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := bt.CheckSession(ctx, tt.input)

			if token != tt.expected {
				t.Errorf("Token mismatch: got %q, want %q", token, tt.expected)
			}

			if (err != nil) && tt.wantErr == nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if (err == nil) && tt.wantErr != nil {
				t.Errorf("Expected error %v, but got nil", tt.wantErr)
			}

			if err != nil && tt.wantErr != nil {
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("Error message mismatch:\n got:  %q\n want to contain: %q", err.Error(), tt.wantErr.Error())
				}
			}
		})
	}
}

func TestSaveExpenseCategory(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()
	userId := "john123"

	tests := []struct {
		name        string
		input       ExpenseCategoryRequest
		expectedMsg string
	}{
		{
			name: "Fail - Empty name",
			input: ExpenseCategoryRequest{
				Name:      "",
				MaxAmount: 3000,
				PeriodDay: 60,
				Note:      "tires, motor",
				Type:      "-",
			},
			expectedMsg: "Category cannot be empty!",
		},
		{
			name: "Fail - Max Amount",
			input: ExpenseCategoryRequest{
				Name:      "Car repair",
				MaxAmount: 9999999999999999999.99,
				PeriodDay: 60,
				Note:      "tires, motor",
				Type:      "-",
			},
			expectedMsg: "maximum amount is too large",
		},
		{
			name: "Fail - Max Category name",
			input: ExpenseCategoryRequest{
				Name:      strings.Repeat("A", 256),
				MaxAmount: 300,
				PeriodDay: 60,
				Note:      "tires, motor",
				Type:      "-",
			},
			expectedMsg: "name so long",
		},
		{
			name: "Success - Valid Expense category",
			input: ExpenseCategoryRequest{
				Name:      "Car repair",
				MaxAmount: 3000,
				PeriodDay: 60,
				Note:      "tires, motor",
				Type:      "-",
			},
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bt.SaveExpenseCategory(ctx, userId, tt.input)

			if tt.expectedMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got nil", tt.expectedMsg)
				}

				var msg string
				if appErr, ok := err.(appErrors.ErrorResponse); ok {
					msg = appErr.Message
				} else {
					msg = err.Error()
				}

				if !strings.Contains(msg, tt.expectedMsg) {
					t.Errorf("Error message mismatch:\n Got:  %q\n Want: %q", msg, tt.expectedMsg)
				}

			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}

}

func TestSaveIncomeCategory(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()
	userId := "john123"

	tests := []struct {
		name        string
		input       IncomeCategoryRequest
		expectedMsg string
	}{
		{
			name: "Fail - Empty name",
			input: IncomeCategoryRequest{
				Name:         "",
				TargetAmount: 3000,
				Note:         "eCommerce",
				Type:         "+",
			},
			expectedMsg: "Category cannot be empty!",
		},
		{
			name: "Fail - Max Target Amount",
			input: IncomeCategoryRequest{
				Name:         "Car repair",
				TargetAmount: math.MaxInt64,
				Note:         "eCommerce",
				Type:         "+",
			},
			expectedMsg: " target amount is too large",
		},
		{
			name: "Fail - Max Category name",
			input: IncomeCategoryRequest{
				Name:         strings.Repeat("A", 256),
				TargetAmount: 3000,
				Note:         "eCommerce",
				Type:         "+",
			},
			expectedMsg: "name so long",
		},
		{
			name: "Success - Valid Expense category",
			input: IncomeCategoryRequest{
				Name:         "Car repair",
				TargetAmount: 3000,
				Note:         "eCommerce",
				Type:         "+",
			},
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bt.SaveIncomeCategory(ctx, userId, tt.input)

			if tt.expectedMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got nil", tt.expectedMsg)
				}

				var msg string
				if appErr, ok := err.(appErrors.ErrorResponse); ok {
					msg = appErr.Message
				} else {
					msg = err.Error()
				}

				if !strings.Contains(msg, tt.expectedMsg) {
					t.Errorf("Error message mismatch:\n Got:  %q\n Want: %q", msg, tt.expectedMsg)
				}

			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}

}

func TestSaveTransaction(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()
	userId := "john123"

	tests := []struct {
		name        string
		input       TransactionRequest
		expectedMsg string
	}{
		{
			name: "Fail - Empty name",
			input: TransactionRequest{
				CategoryName: "",
				CategoryType: "-",
				Amount:       30.33,
				Currency:     "USD",
				Note:         "tires replaced",
			},
			expectedMsg: "Category name cannot be empty!",
		},
		{
			name: "Fail - Zero Amount with decimal",
			input: TransactionRequest{
				CategoryName: "Salary",
				CategoryType: "-",
				Amount:       0.0,
				Currency:     "USD",
				Note:         "tires replaced",
			},
			expectedMsg: "amount is zero or very close to zero",
		},
		{
			name: "Fail - Zero Amount",
			input: TransactionRequest{
				CategoryName: "Salary",
				CategoryType: "-",
				Amount:       0,
				Currency:     "USD",
				Note:         "tires replaced",
			},
			expectedMsg: "amount is zero or very close to zero",
		},
		{
			name: "Fail - Maximum Amount",
			input: TransactionRequest{
				CategoryName: "Salary",
				CategoryType: "-",
				Amount:       math.MaxUint64,
				Currency:     "USD",
				Note:         "tires replaced",
			},
			expectedMsg: "allowed amount per transaction",
		},
		{
			name: "Fail - Long category name",
			input: TransactionRequest{
				CategoryName: strings.Repeat("A", 256),
				CategoryType: "-",
				Amount:       3000,
				Currency:     "USD",
				Note:         "eCommerce",
			},
			expectedMsg: "Category name so long",
		},
		{
			name: "Fail - Long Currency name",
			input: TransactionRequest{
				CategoryName: "Salary",
				CategoryType: "-",
				Amount:       3000,
				Currency:     strings.Repeat("A", 256),
				Note:         "eCommerce",
			},
			expectedMsg: "Currency so long",
		},
		{
			name: "Fail - Long Note",
			input: TransactionRequest{
				CategoryName: "Salary",
				CategoryType: "-",
				Amount:       3000,
				Currency:     "USD",
				Note:         strings.Repeat("A", 1001),
			},
			expectedMsg: "Note so long",
		},
		{
			name: "Success - Valid transaction",
			input: TransactionRequest{
				CategoryName: "Salary",
				CategoryType: "+",
				Amount:       3000,
				Currency:     "USD",
				Note:         "work work work",
			},
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bt.SaveTransaction(ctx, userId, tt.input)

			if tt.expectedMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got nil", tt.expectedMsg)
				}

				var msg string
				if appErr, ok := err.(appErrors.ErrorResponse); ok {
					msg = appErr.Message
				} else {
					msg = err.Error()
				}

				if !strings.Contains(msg, tt.expectedMsg) {
					t.Errorf("Error message mismatch:\n Got:  %q\n Want: %q", msg, tt.expectedMsg)
				}

			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}
}

func TestUpdateExpenseCategory(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()
	userId := "john123"

	tests := []struct {
		name        string
		input       UpdateExpenseCategoryRequest
		expectedMsg string
	}{
		{
			name: "Fail - New max amount",
			input: UpdateExpenseCategoryRequest{
				ID:           "123",
				NewName:      "Holiday",
				NewMaxAmount: 9999999999999999999.99,
				NewPeriodDay: 30,
				NewNote:      "work work",
				UpdateTime:   time.Now(),
			},
			expectedMsg: "Category new maximum amount is too larger",
		},
		{
			name: "Fail - New max name",
			input: UpdateExpenseCategoryRequest{
				ID:           "123",
				NewName:      strings.Repeat("A", 256),
				NewMaxAmount: 40_000,
				NewPeriodDay: 30,
				NewNote:      "work work",
				UpdateTime:   time.Now(),
			},
			expectedMsg: "Category new name so long",
		},
		{
			name: "Fail - New max note",
			input: UpdateExpenseCategoryRequest{
				ID:           "123",
				NewName:      "test",
				NewMaxAmount: 40_000,
				NewPeriodDay: 30,
				NewNote:      strings.Repeat("A", 1001),
				UpdateTime:   time.Now(),
			},
			expectedMsg: "Category new note so long",
		},
		{
			name: "Success - Valid update expense category",
			input: UpdateExpenseCategoryRequest{
				ID:           "123",
				NewName:      "test",
				NewMaxAmount: 40_000,
				NewPeriodDay: 15,
				NewNote:      "work",
				UpdateTime:   time.Now(),
			},
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bt.UpdateExpenseCategory(ctx, userId, tt.input)

			if tt.expectedMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got nil", tt.expectedMsg)
				}

				var msg string
				if appErr, ok := err.(appErrors.ErrorResponse); ok {
					msg = appErr.Message
				} else {
					msg = err.Error()
				}

				if !strings.Contains(msg, tt.expectedMsg) {
					t.Errorf("Error message mismatch:\n Got:  %q\n Want: %q", msg, tt.expectedMsg)
				}

			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}
}

func TestUpdateIncomeCategory(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()
	userId := "john123"

	tests := []struct {
		name        string
		input       UpdateIncomeCategoryRequest
		expectedMsg string
	}{
		{
			name: "Fail - New max target amount",
			input: UpdateIncomeCategoryRequest{
				ID:              "123",
				NewName:         "Holiday",
				NewTargetAmount: math.MaxInt64,
				NewNote:         "work work",
				UpdateTime:      time.Now(),
			},
			expectedMsg: "Category new target amount is too larger",
		},
		{
			name: "Fail - New max name",
			input: UpdateIncomeCategoryRequest{
				ID:              "123",
				NewName:         strings.Repeat("A", 256),
				NewTargetAmount: 3021,
				NewNote:         "work work",
				UpdateTime:      time.Now(),
			},
			expectedMsg: "Category new name so long",
		},
		{
			name: "Fail - New max note",
			input: UpdateIncomeCategoryRequest{
				ID:              "123",
				NewName:         "abc",
				NewTargetAmount: 3021,
				NewNote:         strings.Repeat("A", 1001),
				UpdateTime:      time.Now(),
			},
			expectedMsg: "Category new note so long",
		},
		{
			name: "Success - Valid update expense category",
			input: UpdateIncomeCategoryRequest{
				ID:              "123",
				NewName:         "abc",
				NewTargetAmount: 3021,
				NewNote:         "work",
				UpdateTime:      time.Now(),
			},
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bt.UpdateIncomeCategory(ctx, userId, tt.input)

			if tt.expectedMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got nil", tt.expectedMsg)
				}

				var msg string
				if appErr, ok := err.(appErrors.ErrorResponse); ok {
					msg = appErr.Message
				} else {
					msg = err.Error()
				}

				if !strings.Contains(msg, tt.expectedMsg) {
					t.Errorf("Error message mismatch:\n Got:  %q\n Want: %q", msg, tt.expectedMsg)
				}

			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	mockStore := &MockStorage{}
	bt := &BudgetTracker{storage: mockStore}
	ctx := context.Background()
	userId := "john123"

	tests := []struct {
		name        string
		input       auth.DeleteUser
		expectedMsg string
	}{
		{
			name: "Fail - Empty password",
			input: auth.DeleteUser{
				Password: "",
			},
			expectedMsg: "Password cannot be empty!",
		},
		{
			name: "Success - with reason",
			input: auth.DeleteUser{
				Password: "1234",
				Reason:   "The app is too complex!",
			},
			expectedMsg: "",
		},
		{
			name: "Success - without reason",
			input: auth.DeleteUser{
				Password: "1234",
			},
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bt.DeleteUser(ctx, userId, tt.input)

			if tt.expectedMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got nil", tt.expectedMsg)
				}

				var msg string
				if appErr, ok := err.(appErrors.ErrorResponse); ok {
					msg = appErr.Message
				} else {
					msg = err.Error()
				}

				if !strings.Contains(msg, tt.expectedMsg) {
					t.Errorf("Error message mismatch:\n Got:  %q\n Want: %q", msg, tt.expectedMsg)
				}

			} else {
				if err != nil {
					t.Errorf("Expected success, but got error: %v", err)
				}
			}
		})
	}
}
