package main


package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	errNotFound     = errors.New("not found")
	errInvalidInput = errors.New("invalid input")
	errAuth         = errors.New("unauthorized")
	accessDenied    = errors.New("access denied")
)

type BudgetTracker struct {
	nextID      int
	storage     Storage
	storageType string
}

func NewBudgetTracker(lastID int, s Storage, sType string) BudgetTracker {
	return BudgetTracker{
		nextID:      lastID + 1,
		storage:     s,
		storageType: sType,
	}
}

type Storage interface {
	SaveTransaction(t Transaction) error
	GetTransactions(userID int) ([]Transaction, error)
	GetTransactionsByType(tType string, userID int) ([]Transaction, error)
	GetTransactionById(ID int) (Transaction, error)
	DeleteTransaction(ID int) error
	UpdateTransaction(t UpdateTransactionItem) error
	SaveUser(newUser User) (err error, errCode int)
	GetUserByUserName(username string, c AuthUser) (User, error)
	ValidateUser(credentials AuthUser) (userID int, err error)
	GenerateApiKey(credentials AuthUser) (apiKey string, err error)
	CalcTotal(tType string, userID int) (string, error)
}

func (bt *BudgetTracker) AddTranscation(amount float64, category string, transcationType string, currency string, credentials AuthUser) error {
	tType := strings.ToLower(transcationType)
	userID, err := bt.storage.ValidateUser(credentials)

	if err != nil {
		return fmt.Errorf("%w: credentials are wrong", errAuth)
	}

	if amount > 7000 {
		return fmt.Errorf("%w: max allowed amount per transcation is: %.2f$", errInvalidInput, 7000.0)
	}
	if tType != "income" && tType != "expense" {
		return fmt.Errorf("%w: allowed transaction types are: income and expense", errInvalidInput)
	}
	if len(category) > 30 {
		return fmt.Errorf("%w: category name is too long. Maximum allowed length is 30 characters.", errInvalidInput)
	}

	now := time.Now()
	t := Transaction{
		ID:          bt.nextID,
		Amount:      amount,
		Currency:    currency,
		Category:    category,
		UpdatedDate: now,
		CreatedDate: now,
		Type:        transcationType,
		CreatedBy:   userID,
	}

	if err := bt.storage.SaveTransaction(t); err != nil {
		return fmt.Errorf("failed to save transaction to db: %w", err)
	}
	bt.nextID++
	return nil
}

func (bt *BudgetTracker) GetTransactionsAsList(credentials AuthUser) ([]Transaction, error) {
	userID, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return nil, fmt.Errorf("%w: credentials are wrong.", errAuth)
	}

	result, err := bt.storage.GetTransactions(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions from db: %w", err)

	}
	return result, nil
}

// Bussiness layer protected.
func (bt *BudgetTracker) GetTransactionsByType(transactionType string, credentials AuthUser) ([]Transaction, error) {
	userID, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return nil, fmt.Errorf("%w: credentials are wrong.", errAuth)
	}

	results, err := bt.storage.GetTransactionsByType(transactionType, userID)
	if err != nil {
		return []Transaction{}, err
	}
	return results, nil
}

func (bt *BudgetTracker) GetTranscationById(ID int, credentials AuthUser) (Transaction, error) {
	userID, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return Transaction{}, fmt.Errorf("%w: credentials are wrong.", errAuth)
	}

	if ID < 0 {
		return Transaction{}, fmt.Errorf("%w: ID must be non-negative(e.g: -66,-1,-3...)", errInvalidInput)
	}

	transaction, err := bt.storage.GetTransactionById(ID)
	if err != nil {
		return Transaction{}, fmt.Errorf("%w: failed to get transaction creator: %w", errInvalidInput, err)
	}

	if userID != transaction.CreatedBy {
		return Transaction{}, fmt.Errorf("%s", accessDenied)
	}

	t, err := bt.storage.GetTransactionById(ID)
	if err != nil {
		return Transaction{}, fmt.Errorf("failed to get transacation by id: %w", err)
	}
	return t, nil
}

func (bt *BudgetTracker) CalcTotal(tType string, credentials AuthUser) (string, error) {
	userID, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return "", fmt.Errorf("%w: credentials are wrong.", errAuth)
	}

	tType = strings.ToLower(tType)
	if tType == "" {
		return "", fmt.Errorf("%w: type is empty", errInvalidInput)
	}
	if tType != "income" && tType != "expense" {
		return "", fmt.Errorf("%w: allowed transaction types: Income/Expense", errInvalidInput)
	}

	total, err := bt.storage.CalcTotal(tType, userID)
	if err != nil {
		return "", fmt.Errorf("failed to calculate total of %ss: %w", tType, err)
	}
	return total, nil
}

func (bt *BudgetTracker) UpdateTransaction(t UpdateTransactionItem, credentials AuthUser) error {
	userID, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return fmt.Errorf("%w: credentials are wrong.", errAuth)
	}

	tItem, err := bt.storage.GetTransactionById(t.ID)
	if err != nil {
		return fmt.Errorf("failed to get transaction's creator: %w", err)
	}

	if userID != tItem.CreatedBy {
		return fmt.Errorf("%w: you cannot update other's transaction.", accessDenied)
	}

	if err := bt.storage.UpdateTransaction(t); err != nil {
		return fmt.Errorf("failed to update transaction, Transaction-ID: %d, error: %w", t.ID, err)
	}
	return nil
}

func (bt *BudgetTracker) DeleteTransaction(ID int, credentials AuthUser) error {
	userID, err := bt.storage.ValidateUser(credentials)
	if err != nil {
		return fmt.Errorf("%w: credentials are wrong.", errAuth)
	}

	tItem, err := bt.storage.GetTransactionById(ID)
	if err != nil {
		return fmt.Errorf("failed to get transaction's creator: %w", err)
	}

	if userID != tItem.CreatedBy {
		return fmt.Errorf("%w: you cannot delete other's transaction.", accessDenied)
	}

	if err := bt.storage.DeleteTransaction(ID); err != nil {
		return fmt.Errorf("failed to delete transaction, Transaction-ID: %d, error: %w", ID, err)
	}
	return nil
}

// User section

func (bt *BudgetTracker) SaveUser(newUser User) (err error, errCode int) {
	err, errCode = bt.storage.SaveUser(newUser)
	if err != nil {
		return err, errCode
	}
	// example logic for register
	// if err := sendOTP(user.email); err != nil {
	// 	return fmt.Errorf("failed to send OTP: %w")
	// }
	return nil, 0
}
