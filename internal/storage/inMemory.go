package storage

import (
	"fmt"
	"strings"
	"time"

	authModel "github.com/fatali-fataliyev/budget_tracker/internal/auth"
	budgetModel "github.com/fatali-fataliyev/budget_tracker/internal/budget"
)

type InMemeoryStorage struct {
	transactions []budgetModel.Transaction
	users        []authModel.User
	sessions     []authModel.Session
}

func NewInternalMemeoryStorage(credentials authModel.UserCredentialsPure) *InMemeoryStorage {
	return &InMemeoryStorage{}
}

func (inMem *InMemeoryStorage) GetStorageType() string {
	return "inmemory"
}

func (inMem *InMemeoryStorage) SaveUser(newUser authModel.User) error {
	inMem.users = append(inMem.users, newUser)
	fmt.Println(inMem.users)
	return nil
}

func (inMem *InMemeoryStorage) ValidateUser(credentials authModel.UserCredentialsPure) (authModel.User, error) {

	for _, user := range inMem.users {
		if user.UserName == credentials.UserName {
			if authModel.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) {
				return user, nil
			} else {
				return authModel.User{}, fmt.Errorf("password is wrong")
			}
		}
	}
	return authModel.User{}, fmt.Errorf("user not found.")
}

func (inMem *InMemeoryStorage) CheckSession(token string) (string, error) {
	for _, session := range inMem.sessions {
		if strings.TrimSpace(session.Token) == strings.TrimSpace(token) {
			if session.ExpireAt.After(time.Now()) {
				return session.UserID, nil
			}
			return "", fmt.Errorf("session expired, login again")
		}
	}
	return "", fmt.Errorf("session not found, login again")
}

func (inMem *InMemeoryStorage) SaveSession(session authModel.Session) error {
	inMem.sessions = append(inMem.sessions, session)
	return nil
}

func (inMem *InMemeoryStorage) SaveTransaction(t budgetModel.Transaction) error {
	inMem.transactions = append(inMem.transactions, t)
	return nil
}

func (inMem *InMemeoryStorage) IsEmailConfirmed(emailAddress string) bool {
	for _, user := range inMem.users {
		if user.Email == emailAddress && user.PendingEmail == "" {
			return true
		}
	}
	return false
}

func (inMem *InMemeoryStorage) GetAllTransactions(userId string) ([]budgetModel.Transaction, error) {
	var result []budgetModel.Transaction
	for _, transaction := range inMem.transactions {
		if transaction.CreatedBy == userId {
			result = append(result, transaction)
		}
	}
	return result, nil
}
func (inMem *InMemeoryStorage) GetTransactionsByType(userId string, transactionType string) ([]budgetModel.Transaction, error) {
	// results := []budgetModel.Transaction{}
	// for _, transaction := range inMem.transactions {
	// 	// if transaction.Type == transactionType && transaction.CreatedBy == userId {
	// 	// 	results = append(results, transaction)
	// 	// }
	// }
	return inMem.transactions, nil
}

func (inMem *InMemeoryStorage) GetTransactionsByCategory(userId string, category string) ([]budgetModel.Transaction, error) {
	results := []budgetModel.Transaction{}
	for _, transaction := range inMem.transactions {
		if transaction.CategoryName == category && transaction.CreatedBy == userId {
			results = append(results, transaction)
		}
	}
	return inMem.transactions, nil
}

func (inMem *InMemeoryStorage) GetTransactionById(userId string, transacationID string) (budgetModel.Transaction, error) {
	var result budgetModel.Transaction
	for _, transaction := range inMem.transactions {
		if transaction.ID == transacationID && transaction.CreatedBy == userId {
			result = transaction
		}
	}
	return result, nil
}

func (inMem *InMemeoryStorage) GetTransactionsByCurrency(userId string, currencyType string) ([]budgetModel.Transaction, error) {
	results := []budgetModel.Transaction{}
	// for _, transaction := range inMem.transactions {
	// 	if transaction.Currency == currencyType && transaction.CreatedBy == userId {
	// 		results = append(results, transaction)
	// 	}
	// }
	return results, nil
}

func (inMem *InMemeoryStorage) GetTotalsByType(tType string, userId string) (string, error) {
	var total int
	// for _, transaction := range inMem.transactions {
	// 	if transaction.Type == tType && transaction.CreatedBy == userId {
	// 		transaction.Amount += float64(total)
	// 	}
	// }
	result := fmt.Sprintf("total %s(s) is: %f", tType, float64(total))
	return result, nil
}

func (inMem *InMemeoryStorage) UpdateTransaction(userId string, transacationItem budgetModel.BudgetTracker) error {
	// for tIdx, t := range inMem.transactions {
	// if t.CreatedBy == userId && t.ID == transacationItem.ID {
	// 	inMem.transactions[tIdx].Amount = t.Amount
	// 	inMem.transactions[tIdx].Currency = t.Currency
	// 	inMem.transactions[tIdx].Category = t.Category
	// 	inMem.transactions[tIdx].UpdatedDate = t.UpdatedDate
	// 	inMem.transactions[tIdx].Type = t.Type
	// 	return nil
	// }
	// }
	return fmt.Errorf("transaction not found: transaction update failed.")
}

func (inMem *InMemeoryStorage) DeleteTransaction(userId string, transacationID string) error {
	for i, transaction := range inMem.transactions {
		if transaction.ID == transacationID && transaction.CreatedBy == userId {
			inMem.transactions = append(inMem.transactions[:i], inMem.transactions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("transaction not found: transaction delete failed.")
}

func (inMem *InMemeoryStorage) FindUserByUserName(username string) (string, error) {
	for _, user := range inMem.users {
		if user.UserName == username {
			about := fmt.Sprintf("fullname: %s, username: %s", user.FullName, user.UserName)
			return about, nil
		}
	}
	return "", fmt.Errorf("user not found")
}
