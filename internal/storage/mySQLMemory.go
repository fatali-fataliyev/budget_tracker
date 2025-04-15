package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
)

type MySQLStorage struct {
	db *sql.DB
}

func NewMySQLStorage(db *sql.DB) *MySQLStorage {
	return &MySQLStorage{db: db}
}

func (mySql *MySQLStorage) SaveUser(user auth.User) error {
	query := "INSERT INTO users (id, username, fullname, nickname, hashed_password, email, pending_email) VALUES (?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, user.ID, user.UserName, user.FullName, user.NickName, user.PasswordHashed, user.Email, user.Email)
	if err != nil {
		return fmt.Errorf("failed to register user: %w", err)
	}
	return nil
}

func (mySql *MySQLStorage) SaveSession(session auth.Session) error {
	query := "INSERT INTO sessions (id, token, created_at, expire_at, user_id) VALUES (?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, session.ID, session.Token, session.CreatedAt, session.ExpireAt, session.UserID)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

func (mySql *MySQLStorage) CheckSession(token string) (string, error) {
	query := `SELECT user_id, expire_at FROM sessions WHERE token = ?`

	var userID string
	var expireAtString string

	err := mySql.db.QueryRow(query, token).Scan(&userID, &expireAtString)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("session not found, login again")
		}
		return "", fmt.Errorf("failed to query session: %w", err)
	}
	expireAt, err := time.Parse("2006-01-02 15:04:05", expireAtString)
	if err != nil {
		return "", fmt.Errorf("failed to parse expire_at")
	}
	if expireAt.Before(time.Now()) {
		return "", fmt.Errorf("session expired, please login again")
	}

	return userID, nil
}

func (mySql *MySQLStorage) SaveTransaction(t budget.Transaction) error {
	query := "INSERT INTO transactions (id, amount, currency, category, created_date, updated_date, type, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, t.ID, t.Amount, t.Currency, t.Category, t.CreatedDate, t.UpdatedDate, t.Type, t.CreatedBy)
	if err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}
	return nil
}

func SafeValueConverter[T any](val *T) interface{} {
	//from tutorialspoint.com

	// T istenilen tipde ola biler, ve onun deyeri nildirse nil qaytarir, yox nil deyilse, pointerdeki value-nu qaytarir.
	if val == nil {
		return nil
	}
	return *val
}

func (mySql *MySQLStorage) GetFilteredTransactions(userID string, filters budget.ListTransactionsFilters) ([]budget.Transaction, error) {
	var query string
	args := []interface{}{
		userID,
	}

	fmt.Println("len of categories: ", len(filters.Categories))
	fmt.Println("categories: ", filters.Categories)
	if filters.IsAllNil {
		query = "SELECT id, amount, currency, category, created_date, updated_date, type, created_by FROM transactions WHERE created_by = ?;"
	} else if len(filters.Categories) == 0 {
		fmt.Println("there isn't any categories.")

		query = `SELECT id, amount, currency, category, created_date, updated_date, type, created_by 
          FROM transactions 
          WHERE created_by = ? 
          AND (? IS NULL OR amount >= ?) 
          AND (? IS NULL OR amount <= ?) 
          AND (? IS NULL OR type = ?)`

		args = append(args,
			SafeValueConverter(filters.MinAmount),
			SafeValueConverter(filters.MinAmount),
			SafeValueConverter(filters.MaxAmount),
			SafeValueConverter(filters.MaxAmount),
			SafeValueConverter(filters.Type),
			SafeValueConverter(filters.Type),
		)
	} else {
		categories := make([]interface{}, len(filters.Categories))
		for idx, category := range filters.Categories {
			categories[idx] = category
		}
		query = `SELECT id, amount, currency, category, created_date, updated_date, type, created_by 
          FROM transactions 
          WHERE created_by = ? 
          AND (? IS NULL OR amount >= ?) 
          AND (? IS NULL OR amount <= ?) 
          AND (? IS NULL OR type = ?) 
          AND category IN (?` + strings.Repeat(",?", len(categories)-1) + `)`

		args = append(args,
			SafeValueConverter(filters.MinAmount),
			SafeValueConverter(filters.MinAmount),
			SafeValueConverter(filters.MaxAmount),
			SafeValueConverter(filters.MaxAmount),
			SafeValueConverter(filters.Type),
			SafeValueConverter(filters.Type),
		)
		args = append(args, categories...)
	}

	fmt.Println("args from DB layer: ", args)
	fmt.Println("query from DB layer: ", query)

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	defer rows.Close()

	var transactions []budget.Transaction
	for rows.Next() {
		var t dbTransaction
		err := rows.Scan(&t.ID, &t.Amount, &t.Currency, &t.Category, &t.CreatedDate, &t.UpdatedDate, &t.Type, &t.CreatedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		createdDate, err := time.Parse("2006-01-02 15:04:05", t.CreatedDate)
		if err != nil {
			return nil, fmt.Errorf("failed to convert created date")
		}
		updatedDate, err := time.Parse("2006-01-02 15:04:05", t.UpdatedDate)
		if err != nil {
			return nil, fmt.Errorf("failed to convert updated date")
		}
		budgetT := budget.Transaction{
			ID:          t.ID,
			Amount:      t.Amount,
			Currency:    t.Currency,
			Category:    t.Category,
			CreatedDate: createdDate,
			UpdatedDate: updatedDate,
			Type:        t.Type,
			CreatedBy:   t.CreatedBy,
		}
		transactions = append(transactions, budgetT)
	}
	return transactions, nil
}

func (mySql *MySQLStorage) GetTotalsByType(tType string, userID string) (string, error) {
	query := "SELECT SUM(amount) FROM transactions WHERE created_by = ? AND type = ?;"
	row := mySql.db.QueryRow(query, userID, tType)

	var total string
	err := row.Scan(&total)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no transactions found")
		}
		return "", fmt.Errorf("failed to scan total: %w", err)
	}
	return total, nil
}

func (mySql *MySQLStorage) GetTransactionById(userID string, transactionId string) (budget.Transaction, error) {

	query := "SELECT id, amount, currency, category, created_date, updated_date, type, created_by FROM transactions WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, transactionId)

	var t dbTransaction
	err := row.Scan(&t.ID, &t.Amount, &t.Currency, &t.Category, &t.CreatedDate, &t.UpdatedDate, &t.Type, &t.CreatedBy)
	if err != nil {
		return budget.Transaction{}, fmt.Errorf("failed to scan transaction: %w", err)
	}

	createdDate, err := time.Parse("2006-01-02 15:04:05", t.CreatedDate)
	if err != nil {
		return budget.Transaction{}, fmt.Errorf("failed to convert created date")
	}
	updatedDate, err := time.Parse("2006-01-02 15:04:05", t.UpdatedDate)
	if err != nil {
		return budget.Transaction{}, fmt.Errorf("failed to convert updated date")
	}
	budgetT := budget.Transaction{
		ID:          t.ID,
		Amount:      t.Amount,
		Currency:    t.Currency,
		Category:    t.Category,
		CreatedDate: createdDate,
		UpdatedDate: updatedDate,
		Type:        t.Type,
		CreatedBy:   t.CreatedBy,
	}

	return budgetT, nil
}

func (mySql *MySQLStorage) ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error) {
	query := "SELECT id, username, fullname, nickname, hashed_password, email FROM users WHERE username = ?;"
	row := mySql.db.QueryRow(query, credentials.UserName)

	var user auth.User
	err := row.Scan(&user.ID, &user.UserName, &user.FullName, &user.NickName, &user.PasswordHashed, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, fmt.Errorf("user not found. please register")
		}
		return auth.User{}, fmt.Errorf("failed to scan user: %w", err)
	}

	fmt.Println("founded user: ", user)
	fmt.Println("are they true? : ", auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain))

	fmt.Printf("hash passwrod from db: %s\n", user.PasswordHashed)

	if auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) != true {
		return auth.User{}, fmt.Errorf("password wrong. please try again")
	}
	return user, nil
}

func (mySql *MySQLStorage) DeleteTransaction(userID string, transactionId string) error {
	query := "DELETE FROM transactions WHERE created_by = ? AND id = ?;"
	result, err := mySql.db.Exec(query, userID, transactionId)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found")
	}
	return nil
}
func (mySql *MySQLStorage) UpdateTransaction(userID string, t budget.UpdateTransactionItem) error {
	query := "UPDATE transactions SET amount = ?, currency = ?, category = ?, updated_date = ?, type = ? WHERE created_by = ? AND id = ?;"
	result, err := mySql.db.Exec(query, t.Amount, t.Currency, t.Category, t.UpdatedDate, t.Type, userID, t.ID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found")
	}
	return nil
}

func (mySql *MySQLStorage) IsUserExists(username string) (bool, error) {
	query := "SELECT 1 FROM users WHERE username = ?;"
	row := mySql.db.QueryRow(query, username)
	err := row.Scan()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to scan user: %w", err)
	}
	return true, nil
}

func (mySql *MySQLStorage) IsEmailConfirmed(emailAddress string) bool {
	query := "SELECT COUNT(*) FROM users WHERE email = ? AND pending_email IS NULL;"
	row := mySql.db.QueryRow(query, emailAddress)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func (mySql *MySQLStorage) LogoutUser(userId string, token string) error {
	query := "UPDATE sessions SET expire_at = NOW() - INTERVAL 1 SECOND WHERE user_id = ? AND token = ?"

	_, err := mySql.db.Exec(query, userId, token)
	if err != nil {
		return fmt.Errorf("failed to update session expiration date: %w", err)
	}

	return nil
}

func (mySql *MySQLStorage) GetStorageType() string {
	return "MySQL"
}
