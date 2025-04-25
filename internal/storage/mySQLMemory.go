package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"github.com/subosito/gotenv"
)

func Init() (*sql.DB, error) {
	err := gotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load env variables for database: %w", err)
	}
	username := os.Getenv("DBUSER")
	password := os.Getenv("DBPASS")
	dbname := os.Getenv("DBNAME")

	logging.Logger.Info("Initializing database...")

	db, err := sql.Open("mysql", username+":"+password+"@tcp(localhost:3306)/"+dbname)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	for err := db.Ping(); err != nil; {
		time.Sleep(2 * time.Second)
	}
	logging.Logger.Info("Connected to database")
	return db, nil
}

type MySQLStorage struct {
	db *sql.DB
}

func NewMySQLStorage(db *sql.DB) *MySQLStorage {
	logging.Logger.Debug("New mysql storage created")
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

func (mySql *MySQLStorage) UpdateSession(user_id string, expireDate time.Time) error {
	query := `UPDATE sessions SET expire_at = ? WHERE user_id = ?`

	res, err := mySql.db.Exec(query, expireDate, user_id)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: session not found", budget.ErrNotFound)
	}
	return nil
}

func (mySql *MySQLStorage) GetSessionByToken(token string) (auth.Session, error) {
	query := `SELECT id, token, created_at, expire_at, user_id FROM sessions WHERE token = ?`

	var dSession dbSession
	err := mySql.db.QueryRow(query, token).Scan(
		&dSession.ID,
		&dSession.Token,
		&dSession.CreatedAt,
		&dSession.ExpireAt,
		&dSession.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Session{}, fmt.Errorf("%w: user id not found", budget.ErrNotFound)
		}
		return auth.Session{}, fmt.Errorf("%w: invalid token: %w", budget.ErrInvalidInput, err)
	}

	createdAt, err := time.Parse("2006-01-02 15:04:05", dSession.CreatedAt)
	if err != nil {
		return auth.Session{}, fmt.Errorf("failed to parse created_at")
	}
	expireAt, err := time.Parse("2006-01-02 15:04:05", dSession.ExpireAt)
	if err != nil {
		return auth.Session{}, fmt.Errorf("failed to parse expire_at")
	}

	session := auth.Session{
		ID:        dSession.ID,
		Token:     dSession.Token,
		CreatedAt: createdAt,
		ExpireAt:  expireAt,
		UserID:    dSession.UserID,
	}

	return session, nil
}

func (mySql *MySQLStorage) CheckSession(token string) (string, error) {
	query := `SELECT user_id, expire_at FROM sessions WHERE token = ?`

	var userID string
	var expireAtString string

	err := mySql.db.QueryRow(query, token).Scan(&userID, &expireAtString)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: session not found, login again", budget.ErrInvalidInput)
		}
		return "", fmt.Errorf("failed to check session: %w", err)
	}

	expireAt, err := time.Parse("2006-01-02 15:04:05", expireAtString)
	if err != nil {
		return "", fmt.Errorf("failed to parse expire_at: %w", err)
	}
	now := time.Now()

	if expireAt.Before(now) {
		return "", fmt.Errorf("%w: session expired, please login again", budget.ErrAuth)
	}
	return userID, nil
}

func (mySql *MySQLStorage) SaveTransaction(t budget.Transaction) error {
	query := "INSERT INTO transactions (id, amount, limit_for_amount, currency, category, created_date, updated_date, type, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, t.ID, t.Amount, t.Limit, t.Currency, t.Category, t.CreatedDate, t.UpdatedDate, t.Type, t.CreatedBy)
	if err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}
	return nil
}

func NilToNullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Valid: true, Float64: *v}
}

func NilToNullString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{Valid: true, String: *v}
}

func (mySql *MySQLStorage) GetFilteredTransactions(userID string, filters *budget.ListTransactionsFilters) ([]budget.Transaction, error) {
	var query string
	args := []interface{}{
		userID,
	}

	if filters.IsAllNil {
		query = "SELECT id, amount, limit_for_amount, currency, category, created_date, updated_date, type, created_by FROM transactions WHERE created_by = ?;"
	} else if len(filters.Categories) == 0 {
		query = `SELECT id, amount, limit_for_amount, currency, category, created_date, updated_date, type, created_by 
          FROM transactions 
          WHERE created_by = ? 
          AND (? IS NULL OR amount >= ?) 
          AND (? IS NULL OR amount <= ?) 
          AND (? IS NULL OR type = ?)`

		args = append(args,
			NilToNullFloat64(filters.MinAmount),
			NilToNullFloat64(filters.MinAmount),
			NilToNullFloat64(filters.MaxAmount),
			NilToNullFloat64(filters.MaxAmount),
			NilToNullString(filters.Type),
			NilToNullString(filters.Type),
		)
	} else {
		categories := make([]interface{}, len(filters.Categories))
		for idx, category := range filters.Categories {
			categories[idx] = category
		}
		query = `SELECT id, amount, limit_for_amount, currency, category, created_date, updated_date, type, created_by 
          FROM transactions 
          WHERE created_by = ? 
          AND (? IS NULL OR amount >= ?) 
          AND (? IS NULL OR amount <= ?) 
          AND (? IS NULL OR type = ?) 
          AND category IN (?` + strings.Repeat(",?", len(categories)-1) + `)`

		args = append(args,
			NilToNullFloat64(filters.MinAmount),
			NilToNullFloat64(filters.MinAmount),
			NilToNullFloat64(filters.MaxAmount),
			NilToNullFloat64(filters.MaxAmount),
			NilToNullString(filters.Type),
			NilToNullString(filters.Type),
		)
		args = append(args, categories...)
	}

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	defer rows.Close()

	var transactions []budget.Transaction
	for rows.Next() {
		var t dbTransaction
		err := rows.Scan(&t.ID, &t.Amount, &t.Limit, &t.Currency, &t.Category, &t.CreatedDate, &t.UpdatedDate, &t.Type, &t.CreatedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		createdDate, err := time.Parse("2006-01-02 15:04:05", t.CreatedDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_date")
		}
		updatedDate, err := time.Parse("2006-01-02 15:04:05", t.UpdatedDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_date")
		}
		var usagePercent int
		if t.Limit != 0 {
			usagePercent = (int(t.Amount) * 100) / int(t.Limit)
		} else {
			usagePercent = 0
		}

		budgetT := budget.Transaction{
			ID:           t.ID,
			Amount:       t.Amount,
			Limit:        t.Limit,
			UsagePercent: usagePercent,
			Currency:     t.Currency,
			Category:     t.Category,
			CreatedDate:  createdDate,
			UpdatedDate:  updatedDate,
			Type:         t.Type,
			CreatedBy:    t.CreatedBy,
		}
		transactions = append(transactions, budgetT)
	}
	return transactions, nil
}

func (mySql *MySQLStorage) GetTotals(userID string, filters budget.GetTotals) (budget.GetTotals, error) {
	var query string
	args := []interface{}{
		userID,
	}
	total := budget.GetTotals{}

	query = `SELECT SUM(amount), type, currency 
		FROM transactions 
		WHERE created_by = ? AND type = ? AND currency = ?
		GROUP BY type, currency;`
	args = append(args, filters.Type, filters.Currency)

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		return budget.GetTotals{}, fmt.Errorf("failed to get totals by type: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&total.Total, &total.Type, &total.Currency)
		if err != nil {
			return budget.GetTotals{}, fmt.Errorf("failed to scan transaction get by total: %w", err)
		}
	}
	return total, nil
}

func (mySql *MySQLStorage) GetTransactionById(userID string, transactionId string) (budget.Transaction, error) {
	query := "SELECT id, amount, limit_for_amount, currency, category, created_date, updated_date, type, created_by FROM transactions WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, transactionId)

	var t dbTransaction
	err := row.Scan(&t.ID, &t.Amount, &t.Limit, &t.Currency, &t.Category, &t.CreatedDate, &t.UpdatedDate, &t.Type, &t.CreatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return budget.Transaction{}, fmt.Errorf("%w: transaction not found", budget.ErrNotFound)
		}
		return budget.Transaction{}, fmt.Errorf("failed to scan transaction: %w", err)
	}

	createdDate, err := time.Parse("2006-01-02 15:04:05", t.CreatedDate)
	if err != nil {
		return budget.Transaction{}, fmt.Errorf("failed to parse created_date")
	}
	updatedDate, err := time.Parse("2006-01-02 15:04:05", t.UpdatedDate)
	if err != nil {
		return budget.Transaction{}, fmt.Errorf("failed to parse updated_date")
	}
	var usagePercent int
	if t.Limit != 0 {
		usagePercent = (int(t.Amount) * 100) / int(t.Limit)
	} else {
		usagePercent = 0
	}
	budgetT := budget.Transaction{
		ID:           t.ID,
		Amount:       t.Amount,
		Limit:        t.Limit,
		UsagePercent: usagePercent,
		Currency:     t.Currency,
		Category:     t.Category,
		CreatedDate:  createdDate,
		UpdatedDate:  updatedDate,
		Type:         t.Type,
		CreatedBy:    t.CreatedBy,
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
			return auth.User{}, fmt.Errorf("%w: user not found, register please", budget.ErrNotFound)
		}
		return auth.User{}, fmt.Errorf("failed to scan user: %w", err)
	}
	if auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) != true {
		return auth.User{}, fmt.Errorf("%w: password is wrong", budget.ErrInvalidInput)
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
		return fmt.Errorf("failed to check: delete status: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: transaction not found", budget.ErrNotFound)
	}
	return nil
}
func (mySql *MySQLStorage) UpdateTransaction(userID string, t budget.UpdateTransactionItem) error {
	query := "UPDATE transactions SET amount = ?, limit_for_amount = ?, currency = ?, category = ?, updated_date = ?, type = ? WHERE created_by = ? AND id = ?;"
	result, err := mySql.db.Exec(query, t.Amount, t.Limit, t.Currency, t.Category, t.UpdatedDate, t.Type, userID, t.ID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check: update status: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: transaction not found", budget.ErrNotFound)
	}
	return nil
}

func (mySql *MySQLStorage) ChangeAmountOfTransaction(userId string, tId string, tType string, amount float64) error {
	var query string
	if tType == "+" {
		query = `
			UPDATE transactions 
			SET amount = amount + ? 
			WHERE id = ? AND created_by = ?
		`
	} else {
		query = `
			UPDATE transactions 
			SET amount = amount - ? 
			WHERE id = ? AND created_by = ?
		`
	}
	_, err := mySql.db.Exec(query, amount, tId, userId)
	if err != nil {
		return fmt.Errorf("failed to update transaction amount: %w", err)
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
	query := "UPDATE sessions SET expire_at = UTC_TIMESTAMP() - INTERVAL 1 SECOND WHERE user_id = ? AND token = ?"

	_, err := mySql.db.Exec(query, userId, token)
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	return nil
}

func (mySql *MySQLStorage) GetStorageType() string {
	return "MySQL"
}
