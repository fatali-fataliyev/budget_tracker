package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appErrors "github.com/fatali-fataliyev/budget_tracker/customErrors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/internal/contextutil"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"github.com/go-sql-driver/mysql"
)

// --- INIT START --- //

func Init() (*sql.DB, error) {
	var db *sql.DB
	var err error
	var dbname string

	username := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASS")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbname = os.Getenv("DB_NAME")
	fullDsn := os.Getenv("FULL_DSN")

	if dbname == "" {
		dbname = "budget_tracker"
	}

	var adminDsn string
	if fullDsn != "" {
		parts := strings.Split(fullDsn, "/")
		adminDsn = strings.Join(parts[:len(parts)-1], "/") + "/"
	} else {
		if username == "" || password == "" || host == "" || port == "" {
			return nil, fmt.Errorf("missing required DB environment variables")
		}
		adminDsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true", username, password, host, port)
	}

	logging.Logger.Info("Connecting to MySQL server for initialization...")
	adminDb, err := sql.Open("mysql", adminDsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open admin mysql handle: %v", err)
	}
	connected := false
	for i := 0; i < 15; i++ {
		if err := adminDb.Ping(); err == nil {
			connected = true
			break
		}
		logging.Logger.Warnf("Database not ready, retrying... (%d/15)", i+1)
		time.Sleep(3 * time.Second)
	}
	if !connected {
		return nil, fmt.Errorf("database unreachable after multiple attempts")
	}

	var dbnameExistence string
	checkDbnameExistQuery := "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?"
	err = adminDb.QueryRow(checkDbnameExistQuery, dbname).Scan(&dbnameExistence)

	if err == sql.ErrNoRows {
		logging.Logger.Infof("Database '%s' does not exist, creating...", dbname)
		createDbSql := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", dbname)
		if _, err := adminDb.Exec(createDbSql); err != nil {
			adminDb.Close()
			return nil, fmt.Errorf("failed to create database: %v", err)
		}
	} else if err != nil {
		adminDb.Close()
		return nil, fmt.Errorf("failed to check database existence: %v", err)
	}

	adminDb.Close()

	var finalDsn string
	if fullDsn != "" {
		finalDsn = fullDsn
	} else {
		finalDsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", username, password, host, port, dbname)
	}

	logging.Logger.Info("Connecting to database...")
	db, err = sql.Open("mysql", finalDsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database handle: %v", err)
	}

	if _, err := db.Exec("SET GLOBAL time_zone = '+00:00'"); err != nil {
		logging.Logger.Warn("failed to set database timezone(UTC+0)")
	}

	logging.Logger.Info("Connected to database successfully")
	logging.Logger.Info("Running migrations...")

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	migrationFiles, err := getMigrationFiles("db/migrations")
	if err != nil {
		return fmt.Errorf("failed to get migration files: %v", err)
	}

	lastAppliedMigration, err := getLastAppliedMigration(db)
	if err != nil {
		return fmt.Errorf("failed to get last applied migration name: %v", err)
	}

	newMigrations := filterNewMigrations(migrationFiles, lastAppliedMigration)

	if len(newMigrations) == 0 {
		logging.Logger.Info("no new migration")
		return nil
	}

	for _, migrationFile := range newMigrations {
		logging.Logger.Info("applying migration: ", migrationFile)
		migrationContent, err := os.ReadFile(filepath.Join("db/migrations/", migrationFile))
		if err != nil {
			return fmt.Errorf("failed to read this '%s' migration file, error: %v", migrationFile, err)
		}

		err = applyMigration(db, migrationFile, string(migrationContent))
		if err != nil {
			return fmt.Errorf("failed to apply this '%s' migration file, error: %v", migrationFile, err)
		}

	}

	logging.Logger.Info("all migrations applied successfully")
	return nil
}

func getMigrationFiles(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var migrationFiles []string
	for _, file := range files {
		if file.IsDir() != true && strings.HasSuffix(file.Name(), ".sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	sort.Strings(migrationFiles)
	return migrationFiles, nil
}

func getLastAppliedMigration(db *sql.DB) (string, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migration (
        id INT AUTO_INCREMENT PRIMARY KEY,
        migration_name VARCHAR(255) NOT NULL UNIQUE,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`)

	if err != nil {
		return "", err
	}

	var lastMigration string
	err = db.QueryRow("SELECT migration_name FROM migration ORDER BY migration_name DESC LIMIT 1").Scan(&lastMigration)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return lastMigration, err
}

func filterNewMigrations(all []string, lastApplied string) []string {
	if lastApplied == "" {
		return all
	}

	var result []string
	for _, migration := range all {
		if migration > lastApplied {
			result = append(result, migration)
		}
	}
	return result
}

func applyMigration(db *sql.DB, name, sqlContent string) error {
	txn, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	statements := strings.Split(sqlContent, ";")

	for _, statement := range statements {
		trimmedStmt := strings.TrimSpace(statement)
		if trimmedStmt == "" {
			continue
		}

		if _, err := txn.Exec(trimmedStmt); err != nil {
			txn.Rollback()
			return fmt.Errorf("migration statement failed: %w\nStatement: %s", err, trimmedStmt)
		}
	}

	if _, err := txn.Exec("INSERT INTO migration (migration_name) VALUES (?)", name); err != nil {
		txn.Rollback()
		return fmt.Errorf("failed to record migration name: %w", err)
	}

	return txn.Commit()
}

type MySQLStorage struct {
	db *sql.DB
}

func NewMySQLStorage(db *sql.DB) *MySQLStorage {
	return &MySQLStorage{db: db}
}

func (mySql *MySQLStorage) SaveUser(ctx context.Context, user auth.User) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "INSERT INTO user (id, username, fullname, hashed_password, email, pending_email) VALUES (?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, user.ID, user.UserName, user.FullName, user.PasswordHashed, user.Email, user.Email)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to save user Storage.SaveUser(), Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Registration failed, try again later.",
		}
	}
	return nil
}

// --- INIT END --- //

func (mySql *MySQLStorage) SaveSession(ctx context.Context, session auth.Session) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "INSERT INTO session (id, token, created_at, expire_at, user_id) VALUES (?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, session.ID, session.Token, session.CreatedAt, session.ExpireAt, session.UserID)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to save session in Storage.SaveSession() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to check session, try again later.",
		}
	}
	return nil
}
func (mySql *MySQLStorage) SaveExpenseCategory(ctx context.Context, category budget.ExpenseCategory) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "INSERT INTO expense_category (id, name, max_amount, period_day, created_at, updated_at, note, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, category.ID, category.Name, category.MaxAmount, category.PeriodDay, category.CreatedAt, category.UpdatedAt, category.Note, category.CreatedBy)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 {
				return appErrors.ErrorResponse{
					Code:    appErrors.ErrConflict,
					Message: "The category already exists.",
				}
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to save expense category in Storage.SaveExpenseCategory() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to save the category, try again later.",
		}
	}
	return nil
}

func (mySql *MySQLStorage) SaveIncomeCategory(ctx context.Context, category budget.IncomeCategory) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "INSERT INTO income_category (id, name, target_amount, created_at, updated_at, note, created_by) VALUES (?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, category.ID, category.Name, category.TargetAmount, category.CreatedAt, category.UpdatedAt, category.Note, category.CreatedBy)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 {
				return appErrors.ErrorResponse{
					Code:    appErrors.ErrConflict,
					Message: "The category already exists.",
				}
			}
		}
		logging.Logger.Errorf("[TraceID=%s] | failed to save expense category in Storage.SaveExpenseCategory() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to save the category, try again later.",
		}
	}
	return nil
}

func (mySql *MySQLStorage) UpdateSession(ctx context.Context, userId string, newExpireDate time.Time) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := `UPDATE session SET expire_at = ? WHERE user_id = ?`
	res, err := mySql.db.Exec(query, newExpireDate, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to update session in Storage.UpdateSession() function | Error: %v", traceID, err)

		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to check session, please try again later.",
		}
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to check affected rows in Storage.UpdateSession() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to check session, please try again later.",
		}
	}

	if rowsAffected == 0 {
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Session does not exist, please login.",
		}
	}

	return nil
}

func (mySql *MySQLStorage) GetSessionByToken(ctx context.Context, token string) (auth.Session, error) {
	query := `SELECT id, token, created_at, expire_at, user_id FROM session WHERE token = ?`
	var dbS dbSession

	err := mySql.db.QueryRow(query, token).Scan(
		&dbS.ID,
		&dbS.Token,
		&dbS.CreatedAt,
		&dbS.ExpireAt,
		&dbS.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Session{}, appErrors.ErrorResponse{
				Code:    appErrors.ErrAuth,
				Message: "Session does not exist, please login.",
			}
		}
		return auth.Session{}, err
	}

	return auth.Session{
		ID:        dbS.ID,
		Token:     dbS.Token,
		CreatedAt: dbS.CreatedAt,
		ExpireAt:  dbS.ExpireAt,
		UserID:    dbS.UserID,
	}, nil
}

func (mySql *MySQLStorage) CheckSession(ctx context.Context, token string) (string, error) {
	query := `SELECT user_id, expire_at FROM session WHERE token = ?`

	var userID string
	var expireAt time.Time
	traceID := contextutil.TraceIDFromContext(ctx)

	err := mySql.db.QueryRow(query, token).Scan(&userID, &expireAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", appErrors.ErrorResponse{
				Code:    appErrors.ErrAuth,
				Message: "Session does not exist, please login.",
			}
		}
		logging.Logger.Errorf("[TraceID=%s] | failed to check session existance in Storage.CheckSession() function | Error: %v", traceID, err)
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to check session, please try again later.",
		}
	}

	now := time.Now().UTC()
	if expireAt.Before(now) {
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Your session expired, please login again.",
		}
	}

	return userID, nil
}

func (mySql *MySQLStorage) isCategoryExists(traceID string, categoryName string, categoryType string) (bool, string, error) {
	switch categoryType {
	case "+":
		incomeQuery := "SELECT name FROM income_category WHERE name = ?;"

		var incomeCategoryName string
		row := mySql.db.QueryRow(incomeQuery, categoryName)
		err := row.Scan(&incomeCategoryName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, "+", nil
			}

			logging.Logger.Errorf("[TraceID=%s] | failed to check income category existence in Storage.isCategoryExist() function | Error: %v", traceID, err)
			return false, "", appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to check category existance",
			}
		}

		incomeCategoryName = strings.ToLower(incomeCategoryName)
		if incomeCategoryName != "" && incomeCategoryName == strings.ToLower(categoryName) {
			return true, "+", nil
		}
	case "-":
		expenseQuery := "SELECT name FROM expense_category WHERE name = ?;"

		var expenseCategoryName string
		row := mySql.db.QueryRow(expenseQuery, categoryName)
		err := row.Scan(&expenseCategoryName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, "-", nil
			}

			logging.Logger.Errorf("[TraceID=%s] | failed to check expense category existence in Storage.isCategoryExist() function | Error: %v", traceID, err)
			return false, "", appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to check category existance",
			}
		}

		expenseCategoryName = strings.ToLower(expenseCategoryName)
		if expenseCategoryName != "" && expenseCategoryName == strings.ToLower(categoryName) {
			return true, "-", nil
		}
	}
	return false, "", appErrors.ErrorResponse{
		Code:    appErrors.ErrInvalidInput,
		Message: "Invalid category type",
	}
}

func (mySql *MySQLStorage) SaveTransaction(ctx context.Context, t budget.Transaction) error {
	traceID := contextutil.TraceIDFromContext(ctx)
	isExist, cType, err := mySql.isCategoryExists(traceID, t.CategoryName, t.CategoryType)
	if err != nil {
		return err
	}

	if isExist {
		if cType != "" {
			query := "INSERT INTO transaction (id, category_name, amount, currency, created_at, note, created_by, category_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
			_, err := mySql.db.Exec(query, t.ID, t.CategoryName, t.Amount, t.Currency, t.CreatedAt, t.Note, t.CreatedBy, cType)
			if err != nil {
				logging.Logger.Errorf("[TraceID=%s] | failed to save transaction in Storage.SaveTransaction() function, | Error: %v", traceID, err)
				return appErrors.ErrorResponse{
					Code:    appErrors.ErrInternal,
					Message: "Failed to save transaction, try again later.",
				}
			}

			return nil
		} else {
			return appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: "Invalid category type for transaction",
			}
		}
	}
	return appErrors.ErrorResponse{
		Code:    appErrors.ErrInvalidInput,
		Message: "The category does not exist, please create the category",
	}
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

func (mySql *MySQLStorage) GetTotalAmountOfTransactions(ctx context.Context, userID string, categoryName string, categoryType string) (float64, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	query := `
		SELECT IFNULL(SUM(amount), 0)
		FROM transaction
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
		logging.Logger.Errorf("[TraceID=%s] | failed to get total amount of '%s' categories in Storage.GetTotalAmountOfTransactions() function | Error: %v", traceID, categoryType, err)
		return 0, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get total amount of transactions, try again later",
		}
	}

	return total, nil
}

func (mySql *MySQLStorage) processIncomeRows(ctx context.Context, rows *sql.Rows, userID string) ([]budget.IncomeCategoryResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	defer rows.Close()

	var categories []budget.IncomeCategoryResponse

	for rows.Next() {
		var category budget.IncomeCategoryResponse

		err := rows.Scan(&category.ID, &category.Name, &category.TargetAmount, &category.CreatedAt, &category.UpdatedAt, &category.Note, &category.CreatedBy)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.processIncomeRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get categories, try again later.",
			}
		}

		categoryAmount, err := mySql.GetTotalAmountOfTransactions(ctx, userID, category.Name, "+")
		if err != nil {
			return nil, err
		}
		category.Amount = categoryAmount

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to iterate rows in Storage.processIncomeRows() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get categories, try again later.",
		}
	}

	return categories, nil
}

func (mySql *MySQLStorage) GetFilteredIncomeCategories(ctx context.Context, userID string, filters *budget.IncomeCategoryList) ([]budget.IncomeCategoryResponse, error) {
	query := "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_category WHERE created_by = ?"
	args := []interface{}{userID}
	traceID := contextutil.TraceIDFromContext(ctx)

	// Without filters (all categories)

	if filters.IsAllNil {
		query += " ORDER BY created_at DESC;"
		rows, err := mySql.db.Query(query, args...)

		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to get all income categories in Storage.GetFilteredIncomeCategories() function | Error: %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get cateogories.",
			}
		}
		categories, err := mySql.processIncomeRows(ctx, rows, userID)

		if err != nil {
			return nil, err
		}
		return categories, nil
	}

	// With filters (Filtered income categories)

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

	query += " ORDER BY created_at DESC;"

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered categories in Storage.GetFilteredIncomeCategories() function | Error: %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get categories.",
		}
	}
	categories, err := mySql.processIncomeRows(ctx, rows, userID)

	if err != nil {
		return nil, err
	}
	return categories, nil
}

func (mySql *MySQLStorage) processExpenseRows(ctx context.Context, rows *sql.Rows, userID string) ([]budget.ExpenseCategoryResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	defer rows.Close()

	var categories []budget.ExpenseCategoryResponse

	for rows.Next() {
		var category budget.ExpenseCategoryResponse

		err := rows.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &category.CreatedAt, &category.UpdatedAt, &category.Note, &category.CreatedBy)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.processExpenseRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get categories, try again later.",
			}
		}

		categoryAmount, err := mySql.GetTotalAmountOfTransactions(ctx, userID, category.Name, "-")
		if err != nil {
			return nil, err
		}
		category.Amount = categoryAmount

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to iterate rows in Storage.processExpenseRows() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get categories, try again later.",
		}
	}

	return categories, nil
}

func (mySql *MySQLStorage) GetExpenseCategoryStats(ctx context.Context, userId string) (budget.ExpenseStatsResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	var statsRaw []dbExpenseStats

	query := `
	SELECT sub.amount_range,
	       COUNT(*) AS count
	FROM (
	    SELECT 
	    CASE 
	        WHEN max_amount <= 500 THEN 'less_than_500'
	        WHEN max_amount <= 1000 THEN 'between_501_1000'
	        ELSE 'greater_than_1000'
	    END AS amount_range
	FROM expense_category
	WHERE created_by = ?
	) sub
	GROUP BY sub.amount_range;
	`

	rows, err := mySql.db.Query(query, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get expense categories with max_amount <= 500 in Storage.GetExpenseCategoryStats() function | Error: %v", traceID, err)
		return budget.ExpenseStatsResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get category statistics, try again later",
		}
	}

	defer rows.Close()

	for rows.Next() {
		var stat dbExpenseStats
		err := rows.Scan(&stat.AmountRange, &stat.Count)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.GetExpenseCategoryStats() function | Error: %v", traceID, err)
			return budget.ExpenseStatsResponse{}, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get category statistics, try again later",
			}
		}
		statsRaw = append(statsRaw, stat)
	}
	if err := rows.Err(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to iterate rows in Storage.GetExpenseCategoryStats() function | Error: %v", traceID, err)
		return budget.ExpenseStatsResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get category statistics, try again later",
		}
	}

	var stats budget.ExpenseStatsResponse
	for _, stat := range statsRaw {
		switch strings.ToLower(stat.AmountRange) {
		case "less_than_500":
			stats.LessThan500 = stat.Count
		case "between_501_1000":
			stats.Between500And1000 = stat.Count
		case "greater_than_1000":
			stats.MoreThan1000 = stat.Count
		}
	}

	return stats, nil
}

func (mySql *MySQLStorage) GetIncomeCategoryStats(ctx context.Context, userId string) (budget.IncomeStatsResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	var statsRaw []dbIncomeStats

	query := `
	SELECT sub.amount_range,
	       COUNT(*) AS count
	FROM (
	    SELECT 
	    CASE 
	        WHEN target_amount <= 500 THEN 'less_than_500'
	        WHEN target_amount <= 1000 THEN 'between_501_1000'
	        ELSE 'greater_than_1000'
	    END AS amount_range
	FROM income_category
	WHERE created_by = ?
	) sub
	GROUP BY sub.amount_range;
	`

	rows, err := mySql.db.Query(query, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get expense categories with target_amount <= 500 in Storage.GetIncomeCategoryStats() function | Error: %v", traceID, err)
		return budget.IncomeStatsResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get category statistics, try again later",
		}
	}

	defer rows.Close()

	for rows.Next() {
		var stat dbIncomeStats
		err := rows.Scan(&stat.AmountRange, &stat.Count)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.GetIncomeCategoryStats() function | Error: %v", traceID, err)
			return budget.IncomeStatsResponse{}, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get category statistics, try again later",
			}
		}
		statsRaw = append(statsRaw, stat)
	}
	if err := rows.Err(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to iterate rows in Storage.GetIncomeCategoryStats() function | Error: %v", traceID, err)
		return budget.IncomeStatsResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get category statistics, try again later",
		}
	}

	var stats budget.IncomeStatsResponse
	for _, stat := range statsRaw {
		switch strings.ToLower(stat.AmountRange) {
		case "less_than_500":
			stats.LessThan500 = stat.Count
		case "between_501_1000":
			stats.Between500And1000 = stat.Count
		case "greater_than_1000":
			stats.MoreThan1000 = stat.Count
		}
	}

	return stats, nil
}
func (mySql *MySQLStorage) GetTransactionStats(ctx context.Context, userId string) (budget.TransactionStatsResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := `
	SELECT 
		IFNULL(SUM(CASE WHEN category_type = '-' THEN amount ELSE 0 END), 0) AS expenses,
		IFNULL(SUM(CASE WHEN category_type = '+' THEN amount ELSE 0 END), 0) AS incomes,
		IFNULL(SUM(amount), 0) AS total
	FROM transaction
	WHERE created_by = ?;
	`

	var stat dbTransactionStats
	err := mySql.db.QueryRow(query, userId).Scan(&stat.Expenses, &stat.Incomes, &stat.Total)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get transaction stats in Storage.GetTransactionStats() function | Error: %v", traceID, err)
		return budget.TransactionStatsResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get transaction statistics, try again later",
		}
	}

	return budget.TransactionStatsResponse{
		Expenses: stat.Expenses,
		Incomes:  stat.Incomes,
		Total:    stat.Total,
	}, nil
}

func (mySql *MySQLStorage) GetFilteredExpenseCategories(ctx context.Context, userID string, filters *budget.ExpenseCategoryList) ([]budget.ExpenseCategoryResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	query := "SELECT id, name, max_amount, period_day, created_at, updated_at, note, created_by FROM expense_category WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += " ORDER BY created_at DESC;"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to get all expense categories in Storage.GetFilteredExpenseCategories() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get categories.",
			}
		}

		categories, err := mySql.processExpenseRows(ctx, rows, userID)

		if err != nil {
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

	query += " ORDER BY created_at DESC;"
	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered expense categories in Storage.GetFilteredExpenseCategories() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get the categories.",
		}
	}
	categories, err := mySql.processExpenseRows(ctx, rows, userID)

	if err != nil {
		return nil, err
	}
	return categories, nil
}

func (mySql *MySQLStorage) UpdateExpenseCategory(ctx context.Context, userID string, filters budget.UpdateExpenseCategoryRequest) (*budget.ExpenseCategoryResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "UPDATE expense_category SET name = ?, max_amount = ?, period_day = ?, updated_at = ?, note = ? WHERE created_by = ? AND id = ?;"
	_, err := mySql.db.Exec(query, filters.NewName, filters.NewMaxAmount, filters.NewPeriodDay, filters.UpdateTime, filters.NewNote, userID, filters.ID)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to update expense category in Storage.UpdateExpenseCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to update the category.",
		}
	}

	query = "SELECT id, name, max_amount, period_day, created_at, updated_at, note, created_by FROM expense_category WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, filters.ID)

	var category budget.ExpenseCategoryResponse

	err = row.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &category.CreatedAt, &category.UpdatedAt, &category.Note, &category.CreatedBy)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.UpdateExpenseCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to update the category.",
		}
	}

	categoryAmount, err := mySql.GetTotalAmountOfTransactions(ctx, userID, category.Name, "-")
	if err != nil {
		return nil, err
	}

	category.Amount = categoryAmount
	return &category, nil
}

func (mySql *MySQLStorage) UpdateIncomeCategory(ctx context.Context, userID string, filters budget.UpdateIncomeCategoryRequest) (*budget.IncomeCategoryResponse, error) {
	query := "UPDATE income_category SET name = ?, target_amount = ?, updated_at = ?, note = ? WHERE created_by = ? AND id = ?;"
	traceID := contextutil.TraceIDFromContext(ctx)

	_, err := mySql.db.Exec(query, filters.NewName, filters.NewTargetAmount, filters.UpdateTime, filters.NewNote, userID, filters.ID)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] |  failed to update income category in Storage.UpdateIncomeCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to update the category.",
		}
	}

	query = "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_category WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, filters.ID)

	var category budget.IncomeCategoryResponse

	err = row.Scan(&category.ID, &category.Name, &category.TargetAmount, &category.CreatedAt, &category.UpdatedAt, &category.Note, &category.CreatedBy)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.UpdateIncomeCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to update the category.",
		}
	}

	categoryAmount, err := mySql.GetTotalAmountOfTransactions(ctx, userID, category.Name, "+")
	if err != nil {
		return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
	}

	category.Amount = categoryAmount
	return &category, nil
}

func (mySql *MySQLStorage) DeleteExpenseCategory(ctx context.Context, userId string, categoryId string) error {
	tx, err := mySql.db.Begin()
	traceID := contextutil.TraceIDFromContext(ctx)

	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] |  failed to start SQL transaction in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	categoryName, err := mySql.getCategoryNameById(traceID, userId, categoryId, "-")
	if err != nil {
		tx.Rollback()
		return err
	}

	deleteTxQuery := "DELETE FROM transaction WHERE created_by = ? AND category_name = ? AND category_type = '-';"
	_, err = tx.Exec(deleteTxQuery, userId, categoryName)
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] |  failed to delete all related transactions in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	deleteCategoryQuery := "DELETE FROM expense_category WHERE created_by = ? AND id = ?;"
	result, err := tx.Exec(deleteCategoryQuery, userId, categoryId)
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] |  failed to delete expense category in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to check expense category delete status in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}

	}
	if rowsAffected == 0 {
		tx.Rollback()
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "The category does not exist.",
		}
	}

	if err := tx.Commit(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to commit  SQL transaction in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	return nil
}

func (mySql *MySQLStorage) getCategoryNameById(traceID string, userID string, categoryId string, categoryType string) (*string, error) {
	var query string

	switch categoryType {
	case "-":
		query = "SELECT name FROM expense_category WHERE created_by = ? AND id = ?;"
	case "+":
		query = "SELECT name FROM income_category WHERE created_by = ? AND id = ?;"
	default:
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Invalid category type.",
		}
	}

	row := mySql.db.QueryRow(query, userID, categoryId)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: "The category does not exists.",
			}
		}
		logging.Logger.Errorf("[TraceID=%s] | failed to get category name by id from Storage.getCategoryNameById() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get category name.",
		}
	}

	return &name, nil
}

func (mySql *MySQLStorage) DeleteIncomeCategory(ctx context.Context, userId string, categoryId string) error {
	tx, err := mySql.db.Begin()
	traceID := contextutil.TraceIDFromContext(ctx)

	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] |  failed to start SQL transaction in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	categoryName, err := mySql.getCategoryNameById(traceID, userId, categoryId, "+")
	if err != nil {
		tx.Rollback()
		return err
	}

	deleteTxQuery := "DELETE FROM transaction WHERE created_by = ? AND category_name = ? AND category_type = '+';"
	_, err = tx.Exec(deleteTxQuery, userId, categoryName)
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] |  failed to delete all related transactions in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	deleteCategoryQuery := "DELETE FROM income_category WHERE created_by = ? AND id = ?;"
	result, err := tx.Exec(deleteCategoryQuery, userId, categoryId)
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to delete income category in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to check income category delete status in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	if rowsAffected == 0 {
		tx.Rollback()
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "The category does not exist.",
		}
	}

	if err := tx.Commit(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] |  failed to commit SQL transaction  in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete the category.",
		}
	}

	return nil
}

func (mySql *MySQLStorage) processTransactionRows(ctx context.Context, rows *sql.Rows) ([]budget.Transaction, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	defer rows.Close()

	var transactions []budget.Transaction

	for rows.Next() {
		var transaction budget.Transaction

		err := rows.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &transaction.CreatedAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.processTransactionRows() | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to process transactions, try again later.",
			}
		}
		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to iterate rows in Storage.processTransactionRows() | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to process transactions, try again later.",
		}
	}

	return transactions, nil
}

func (mySql *MySQLStorage) GetFilteredTransactions(ctx context.Context, userID string, filters *budget.TransactionList) ([]budget.Transaction, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	query := "SELECT id, amount, currency, created_at, note, created_by, category_type FROM transaction WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += " ORDER BY created_at DESC;"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to get all transactions from Storage.GetFilteredTransactions() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "Failed to get transactions, try again later.",
			}
		}

		transactions, err := mySql.processTransactionRows(ctx, rows)
		if err != nil {
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

	query += " ORDER BY created_at DESC;"
	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered transactions from Storage.GetFilteredTransactions() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get transactions, try again later.",
		}
	}

	transactions, err := mySql.processTransactionRows(ctx, rows)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

func (mySql *MySQLStorage) GetTransactionById(ctx context.Context, userID string, transactionId string) (budget.Transaction, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "SELECT id, category_name, amount, currency, created_at, note, created_by, category_type FROM transaction WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, transactionId)
	var transaction budget.Transaction
	err := row.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &transaction.CreatedAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return budget.Transaction{}, appErrors.ErrorResponse{
				Code:    appErrors.ErrInvalidInput,
				Message: "The category does not exist.",
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.GetTransactionById() function | Error : %v", traceID, err)
		return budget.Transaction{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get transcation",
		}
	}

	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at field in Storage.GetTransactionById() function | Error : %v", traceID, err)
		return budget.Transaction{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get transcation",
		}
	}

	return transaction, nil
}

func (mySql *MySQLStorage) ValidateUser(ctx context.Context, credentials auth.UserCredentialsPure) (auth.User, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := "SELECT id, username, fullname, hashed_password, email FROM user WHERE username = ?;"
	row := mySql.db.QueryRow(query, credentials.UserName)
	var user auth.User
	err := row.Scan(&user.ID, &user.UserName, &user.FullName, &user.PasswordHashed, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, appErrors.ErrorResponse{
				Code:    appErrors.ErrAuth,
				Message: "Username or Password is incorrect",
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to scan user row in Storage.ValidateUser() function | Error : %v", traceID, err)
		return auth.User{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "UNKNOWN",
		}
	}
	if auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) != true {
		return auth.User{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username or Password is incorrect",
		}
	}

	return user, nil
}

func (mySql *MySQLStorage) IsUserExists(ctx context.Context, username string) (bool, error) {
	query := "SELECT 1 FROM user WHERE username = ?;"

	var dummy int
	row := mySql.db.QueryRow(query, username)
	err := row.Scan(&dummy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		traceID := contextutil.TraceIDFromContext(ctx)
		logging.Logger.Errorf("[TraceID=%s] | failed to check user existance in Storage.IsUserExists() function |  Error: %v", traceID, err)
		return false, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to check user existance, try again later.",
		}
	}

	return true, nil
}

func (mySql *MySQLStorage) IsEmailConfirmed(ctx context.Context, emailAddress string) (bool, error) {
	query := "SELECT COUNT(*) FROM user WHERE email = ? AND pending_email IS NULL;"
	row := mySql.db.QueryRow(query, emailAddress)
	traceID := contextutil.TraceIDFromContext(ctx)
	var count int
	err := row.Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to check email confirmation in Storage.IsEmailConfirmed() function | Error: %v", traceID, err)
		return false, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to check email address, try again later.",
		}
	}

	return count > 0, nil
}

func (mySql *MySQLStorage) LogoutUser(ctx context.Context, userId string, token string) error {
	traceID := contextutil.TraceIDFromContext(ctx)
	query := "UPDATE session SET expire_at = UTC_TIMESTAMP() - INTERVAL 1 SECOND WHERE user_id = ? AND token = ?"

	_, err := mySql.db.Exec(query, userId, token)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to logout user in Storage.LogoutUser() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to logout, try again later.",
		}
	}

	return nil
}

func (mySql *MySQLStorage) GetUserData(ctx context.Context, userId string) (budget.UserDataResponse, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	expenseCategories, err := mySql.GetFilteredExpenseCategories(ctx, userId, &budget.ExpenseCategoryList{IsAllNil: true})
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered expense categories in Storage.GetUserData() function | Error: %v", traceID, err)
		return budget.UserDataResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get account info, try later.",
		}
	}
	incomeCategories, err := mySql.GetFilteredIncomeCategories(ctx, userId, &budget.IncomeCategoryList{IsAllNil: true})
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered income categories in Storage.GetUserData() function | Error: %v", traceID, err)
		return budget.UserDataResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get account info, try later.",
		}
	}
	transactions, err := mySql.GetFilteredTransactions(ctx, userId, &budget.TransactionList{IsAllNil: true})
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered transactions in Storage.GetUserData() function | Error: %v", traceID, err)
		return budget.UserDataResponse{}, appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to get account info, try later.",
		}
	}

	userData := budget.UserDataResponse{
		ExpenseCategories: expenseCategories,
		IncomeCategories:  incomeCategories,
		Transactions:      transactions,
	}

	return userData, nil
}

func (mySql *MySQLStorage) DeleteUser(ctx context.Context, userId string, deleteReq auth.DeleteUser) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	txn, err := mySql.db.Begin()
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to start SQL transaction in Storage.DeleteUser() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	var hashedPassword string
	passwordQuery := "SELECT hashed_password FROM user WHERE id = ?;"
	row := mySql.db.QueryRow(passwordQuery, userId)

	if err := row.Scan(&hashedPassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.ErrorResponse{
				Code:    appErrors.ErrInternal,
				Message: "User does not exist.",
			}
		}

		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to scan user row in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	if auth.ComparePasswords(hashedPassword, deleteReq.Password) != true {
		txn.Rollback()
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Username or Password is incorrect",
		}
	}

	sessionDelQuery := "DELETE FROM session WHERE user_id = ?;"
	_, err = mySql.db.Exec(sessionDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s]| failed to delete all user sessions in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	txnDelQuery := "DELETE FROM transaction WHERE created_by = ?;"
	_, err = mySql.db.Exec(txnDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to delete all user transactions in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	incomeDelQuery := "DELETE FROM income_category WHERE created_by = ?;"
	_, err = mySql.db.Exec(incomeDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to delete all user income categories in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	expenseDelQuery := "DELETE FROM expense_category WHERE created_by = ?;"
	_, err = mySql.db.Exec(expenseDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s]| failed to delete all user expense categories in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	userDelQuery := "DELETE FROM user where id = ?;"
	_, err = mySql.db.Exec(userDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s]| failed to delete user in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	deleteInfoQuery := "INSERT INTO deleted_account (reason) VALUES (?);"
	_, err = mySql.db.Exec(deleteInfoQuery, deleteReq.Reason)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to insert delete info in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to delete account, try later.",
		}
	}

	return nil
}

func (mySql *MySQLStorage) GetAccountInfo(ctx context.Context, userId string) (budget.AccountInfo, error) {
	var info budget.AccountInfo

	query := `
		SELECT username, fullname, email, joined_at
		FROM user
		WHERE id = ?
	`

	row := mySql.db.QueryRowContext(ctx, query, userId)
	err := row.Scan(&info.Username, &info.Fullname, &info.Email, &info.JoinedAt)
	if err != nil {
		return budget.AccountInfo{}, fmt.Errorf("GetAccountInfo: %w", err)
	}

	return info, nil
}

func (mySql *MySQLStorage) GetStorageType() string {
	return "MySQL"
}
