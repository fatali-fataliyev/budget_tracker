package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"github.com/go-sql-driver/mysql"
	"github.com/subosito/gotenv"
)

func Init() (*sql.DB, error) {
	err := gotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load 'env' variables for database: %w", err)
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
	query := "INSERT INTO users (id, username, fullname, hashed_password, email, pending_email) VALUES (?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, user.ID, user.UserName, user.FullName, user.PasswordHashed, user.Email, user.Email)
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
func (mySql *MySQLStorage) SaveExpenseCategory(category budget.ExpenseCategory) error {
	if category.Type == "" {
		return fmt.Errorf("%w: category type is empty", appErrors.ErrInvalidInput)
	}
	if category.Type == "-" {
		query := "INSERT INTO expense_categories (id, name, max_amount, period_day, created_at, updated_at, note, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
		_, err := mySql.db.Exec(query, category.ID, category.Name, category.MaxAmount, category.PeriodDay, category.CreatedAt, category.UpdatedAt, category.Note, category.CreatedBy)
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok {
				if mysqlErr.Number == 1062 {
					return fmt.Errorf("%w: this category already exist", appErrors.ErrConflict)
				}
			}
			return fmt.Errorf("failed to save category: %w", err)
		}
		return nil
	}
	return fmt.Errorf("%w: category type is not valid", appErrors.ErrInvalidInput)
}

func (mySql *MySQLStorage) SaveIncomeCategory(category budget.IncomeCategory) error {
	if category.Type == "" {
		return fmt.Errorf("%w: category type is empty", appErrors.ErrInvalidInput)
	}
	if category.Type == "+" {
		query := "INSERT INTO income_categories (id, name, target_amount, created_at, updated_at, note, created_by) VALUES (?, ?, ?, ?, ?, ?, ?);"
		_, err := mySql.db.Exec(query, category.ID, category.Name, category.TargetAmount, category.CreatedAt, category.UpdatedAt, category.Note, category.CreatedBy)
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok {
				if mysqlErr.Number == 1062 {
					return fmt.Errorf("%w: this category already exist", appErrors.ErrConflict)
				}
			}
			return fmt.Errorf("failed to save category: %w", err)
		}
		return nil
	}
	return fmt.Errorf("%w: category type is not valid", appErrors.ErrInvalidInput)
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
		return fmt.Errorf("%w: session not found", appErrors.ErrNotFound)
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
			return auth.Session{}, fmt.Errorf("%w: session not found", appErrors.ErrNotFound)
		}
		return auth.Session{}, fmt.Errorf("failed to scan session row: %w", err)
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
			return "", fmt.Errorf("%w: session not found", appErrors.ErrNotFound)
		}
		return "", fmt.Errorf("failed to check session: %w", err)
	}

	expireAt, err := time.Parse("2006-01-02 15:04:05", expireAtString)
	if err != nil {
		return "", fmt.Errorf("failed to parse expire_at: %w", err)
	}
	now := time.Now()

	if expireAt.Before(now) {
		return "", fmt.Errorf("%w: session expired, please login again", appErrors.ErrAuth)
	}
	return userID, nil
}

func (mySql *MySQLStorage) GetCategoryTypeByName(userId string, categoryName string) (string, error) {
	query := "SELECT id FROM expense_categories WHERE created_by = ? AND name = ?;"
	row := mySql.db.QueryRow(query, userId, categoryName)

	var categoryType string
	var id string
	err := row.Scan(&id)
	if err == nil {
		categoryType = "-"
		return categoryType, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to scan expense category type: %w", err)
	}

	query = "SELECT id FROM income_categories WHERE created_by = ? AND name = ?;"
	row = mySql.db.QueryRow(query, userId, categoryName)

	err = row.Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: category not found", appErrors.ErrNotFound)
		}
		return "", fmt.Errorf("failed to scan income category type: %w", err)
	}
	categoryType = "+"
	return categoryType, nil
}

func (mySql *MySQLStorage) SaveTransaction(t budget.Transaction) error {
	query := "INSERT INTO transactions (id, category_name, amount, currency, created_at, note, created_by, category_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
	categoryType, err := mySql.GetCategoryTypeByName(t.CreatedBy, t.CategoryName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: category not found", appErrors.ErrNotFound)
		}
		return fmt.Errorf("failed to get category type: %w", err)
	}
	t.CategoryType = categoryType
	_, err = mySql.db.Exec(query, t.ID, t.CategoryName, t.Amount, t.Currency, t.CreatedAt, t.Note, t.CreatedBy, t.CategoryType)
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

func (mySql *MySQLStorage) GetTotalAmountOfTransactions(userID string, categoryName string, categoryType string) (float64, error) {
	query := `
		SELECT IFNULL(SUM(amount), 0)
		FROM transactions
		WHERE created_by = ?
	`

	args := []interface{}{userID}
	if categoryName != "" {
		query += " AND category_name = ?"
		args = append(args, categoryName)
	}

	if categoryType != "" {
		query += " AND category_type = ?"
		args = append(args, categoryType)
	}

	var total float64
	err := mySql.db.QueryRow(query, args...).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (mySql *MySQLStorage) GetFilteredIncomeCategories(userID string, filters *budget.IncomeCategoryList) ([]budget.IncomeCategoryResponse, error) {
	query := "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_categories WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			return nil, err
		}

		defer rows.Close()

		var categories []budget.IncomeCategoryResponse
		for rows.Next() {
			var category budget.IncomeCategoryResponse
			var createdAt string
			var updatedAt string

			err = rows.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
			fmt.Println(category.TargetAmount)
			if err != nil {
				return nil, err
			}
			category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse created_at: %w", err)
			}
			category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse updated_at: %w", err)
			}

			categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "+")
			if err != nil {
				return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
			}
			category.Amount = categoryAmount
			fmt.Println(category.TargetAmount)
			categories = append(categories, category)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return categories, nil
	}

	if len(filters.Names) > 0 {
		query += " AND name IN (?" + strings.Repeat(",?", len(filters.Names)-1) + ")"
		for _, name := range filters.Names {
			args = append(args, name)
		}
	}

	if filters.TargetAmount > 0 {
		query += " AND target_amount <= ?"
		args = append(args, filters.TargetAmount)
	}

	if !filters.CreatedAt.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filters.CreatedAt)
	}

	if !filters.EndDate.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filters.EndDate)
	}

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var categories []budget.IncomeCategoryResponse
	for rows.Next() {
		var category budget.IncomeCategoryResponse
		var createdAt string
		var updatedAt string

		err = rows.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
		fmt.Println(category.TargetAmount)
		if err != nil {
			return nil, err
		}
		category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}

		categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "+")
		if err != nil {
			return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
		}
		category.Amount = categoryAmount
		fmt.Println(category.TargetAmount)
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

func (mySql *MySQLStorage) GetFilteredExpenseCategories(userID string, filters *budget.ExpenseCategoryList) ([]budget.ExpenseCategoryResponse, error) {
	query := "SELECT id, name, max_amount, period_day, created_at, updated_at, note, created_by FROM expense_categories WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			return nil, err
		}

		defer rows.Close()

		var categories []budget.ExpenseCategoryResponse
		for rows.Next() {
			var category budget.ExpenseCategoryResponse
			var createdAt string
			var updatedAt string

			err = rows.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
			if err != nil {
				return nil, err
			}
			category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse created_at: %w", err)
			}
			category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse updated_at: %w", err)
			}

			categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "-")
			if err != nil {
				return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
			}
			category.Amount = categoryAmount

			categories = append(categories, category)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return categories, nil
	}

	if filters.PeriodDay > 0 {
		query += " AND period_day >= ?"
		args = append(args, filters.PeriodDay)
	}

	if len(filters.Names) > 0 {
		query += " AND name IN (?" + strings.Repeat(",?", len(filters.Names)-1) + ")"
		for _, name := range filters.Names {
			args = append(args, name)
		}
	}

	if filters.MaxAmount > 0 {
		query += " AND max_amount <= ?"
		args = append(args, filters.MaxAmount)
	}

	if !filters.CreatedAt.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filters.CreatedAt)
	}

	if !filters.EndDate.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filters.EndDate)
	}

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []budget.ExpenseCategoryResponse
	for rows.Next() {
		var category budget.ExpenseCategoryResponse
		var createdAt string
		var updatedAt string

		err = rows.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
		if err != nil {
			return nil, err
		}
		category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}

		categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "-")
		if err != nil {
			return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
		}
		category.Amount = categoryAmount

		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

func (mySql *MySQLStorage) GetFilteredTransactions(userID string, filters *budget.TransactionList) ([]budget.Transaction, error) {
	query := "SELECT id, category_name, amount, currency, created_at, note, created_by, category_type FROM transactions WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			fmt.Print("Error when getting transactions: ", err)
			return nil, err
		}

		defer rows.Close()

		var transactions []budget.Transaction
		for rows.Next() {
			var transaction budget.Transaction
			var createdAt string

			err = rows.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &createdAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
			if err != nil {
				fmt.Print("Error when scanning transaction: ", err)
				return nil, err
			}
			transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse created_at of transaction: %w", err)
			}
			transactions = append(transactions, transaction)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return transactions, nil
	}

	if len(filters.CategoryNames) > 0 {
		query += " AND category_name IN (?" + strings.Repeat(",?", len(filters.CategoryNames)-1) + ")"
		for _, name := range filters.CategoryNames {
			args = append(args, name)
		}
	}

	if filters.Amount > 0 {
		query += " AND amount >= ?"
		args = append(args, filters.Amount)
	}

	if !filters.CreatedAt.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filters.CreatedAt)
	}

	if filters.Currency != "" {
		query += " AND currency = ?"
		args = append(args, filters.Currency)
	}

	if filters.Type != "" {
		query += " AND category_type = ?"
		args = append(args, filters.Type)
	}

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []budget.Transaction
	for rows.Next() {
		var transaction budget.Transaction
		var createdAt string

		err = rows.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &createdAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
		if err != nil {
			return nil, err
		}
		transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at of transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (mySql *MySQLStorage) GetTransactionById(userID string, transactionId string) (budget.Transaction, error) {
	query := "SELECT id, category_name, amount, currency, created_at, note, created_by, category_type FROM transactions WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, transactionId)

	var transaction budget.Transaction
	var createdAt string
	err := row.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &createdAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return budget.Transaction{}, fmt.Errorf("%w: transaction not found", appErrors.ErrNotFound)
		}
		return budget.Transaction{}, fmt.Errorf("failed to scan transaction: %w", err)
	}

	transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		return budget.Transaction{}, fmt.Errorf("failed to parse created_date")
	}

	return transaction, nil
}

func (mySql *MySQLStorage) ValidateUser(credentials auth.UserCredentialsPure) (auth.User, error) {
	query := "SELECT id, username, fullname, hashed_password, email FROM users WHERE username = ?;"
	row := mySql.db.QueryRow(query, credentials.UserName)

	var user auth.User
	err := row.Scan(&user.ID, &user.UserName, &user.FullName, &user.PasswordHashed, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, fmt.Errorf("%w: user not found, register please", appErrors.ErrNotFound)
		}
		return auth.User{}, fmt.Errorf("failed to scan user: %w", err)
	}
	if auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) != true {
		return auth.User{}, fmt.Errorf("%w: password is wrong", appErrors.ErrInvalidInput)
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
		return fmt.Errorf("%w: transaction not found", appErrors.ErrNotFound)
	}
	return nil
}
func (mySql *MySQLStorage) UpdateTransaction(userID string, t budget.Transaction) error {
	// query := "UPDATE transactions SET amount = ?, limit_for_amount = ?, currency = ?, category = ?, updated_date = ?, type = ? WHERE created_by = ? AND id = ?;"
	// result, err := mySql.db.Exec(query, t.Amount, t.Limit, t.Currency, t.Category, t.UpdatedDate, t.Type, userID, t.ID)
	// if err != nil {
	// return fmt.Errorf("failed to update transaction: %w", err)
	// }

	// rowsAffected, err := result.RowsAffected()
	// if err != nil {
	// 	return fmt.Errorf("failed to check: update status: %w", err)
	// }
	// if rowsAffected == 0 {
	// 	return fmt.Errorf("%w: transaction not found", appErrors.ErrNotFound)
	// }
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
