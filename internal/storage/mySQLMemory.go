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

	appErrors "github.com/fatali-fataliyev/budget_tracker/errors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/internal/contextutil"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/subosito/gotenv"
)

// --- INIT START --- //

func Init() (*sql.DB, error) {
	err := gotenv.Load()
	if err != nil {
		logging.Logger.Warn("failed to load '.env' file variables, exptecting they are set in environment...")
	}

	username := os.Getenv("DBUSER")
	password := os.Getenv("DBPASS")
	dbname := os.Getenv("DBNAME")

	logging.Logger.Info("connecting to root MySQL...")

	dsnWithoutDb := fmt.Sprintf("%s:%s@tcp(localhost:3306)/", username, password)
	rootDb, err := sql.Open("mysql", dsnWithoutDb)
	if err != nil {
		logging.Logger.Errorf("failed to connect root mysql: %v", err)
		return nil, fmt.Errorf("failed to connect root mysql")
	}
	defer rootDb.Close()
	logging.Logger.Info("connected to root MySQL")

	var isFirstTime bool

	var exists string
	checkQuery := "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?"
	err = rootDb.QueryRow(checkQuery, dbname).Scan(&exists)
	if err == sql.ErrNoRows {
		logging.Logger.Infof("database '%s' does not exist, creating...", dbname)

		createDbSql := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", dbname)
		_, err = rootDb.Exec(createDbSql)
		if err != nil {
			logging.Logger.Errorf("failed to create database: %v", err)
			return nil, fmt.Errorf("failed to create database")
		}

		isFirstTime = true
	} else if err != nil {
		logging.Logger.Errorf("failed to check database existence: %v", err)
		return nil, fmt.Errorf("failed to check database existence")
	} else {
		logging.Logger.Infof("Database [%s] already exists", dbname)
	}

	dbTimezoneSql := "SET GLOBAL time_zone = '+00:00'"
	_, err = rootDb.Exec(dbTimezoneSql)
	if err != nil {
		logging.Logger.Errorf("failed to set timezone: %v", err)
		return nil, fmt.Errorf("failed to set timezone")
	}

	logging.Logger.Info("connecting to database")
	dsnWithDb := fmt.Sprintf("%s:%s@tcp(localhost:3306)/%s?multiStatements=true", username, password, dbname)
	db, err := sql.Open("mysql", dsnWithDb)
	if err != nil {
		logging.Logger.Errorf("failed to open database: %v", err)
		return nil, fmt.Errorf("failed to open database")
	}
	logging.Logger.Info("connected to database")
	logging.Logger.Info("ping database...")

	var pingCounter = 0
	for i := 0; i < 10; i++ {
		pingCounter++
		if err := db.Ping(); err == nil {
			break
		}
		logging.Logger.Warnf("waiting for database...")
		time.Sleep(3 * time.Second)
	}
	if pingCounter == 10 {
		logging.Logger.Errorf("failed to ping database after 10 attempts: %v", err)
		return nil, fmt.Errorf("failed to ping database")
	}

	logging.Logger.Info("ping response is positive")
	logging.Logger.Info("running migrations")

	migrationFiles, err := getMigrationFiles("db/migrations")
	if err != nil {
		logging.Logger.Errorf("failed to get migration files: %v", err)
		return nil, fmt.Errorf("failed to run migrations")
	}

	lastAppliedMigration, err := getLastAppliedMigration(db, isFirstTime)
	if err != nil {
		logging.Logger.Errorf("failed to get last applied migration name: %v", err)
		return nil, nil
	}

	newMigrations := filterNewMigrations(migrationFiles, lastAppliedMigration)

	if len(newMigrations) == 0 {
		logging.Logger.Info("no new migration")
		return db, nil
	}

	for _, migrationFile := range newMigrations {
		logging.Logger.Info("applying migration: ", migrationFile)
		migrationContent, err := os.ReadFile(filepath.Join("db/migrations/", migrationFile))
		if err != nil {
			logging.Logger.Errorf("failed to read this '%s' migration file, error: %v", migrationFile, err)
			return nil, fmt.Errorf("failed to run migrations")
		}

		err = applyMigration(db, migrationFile, string(migrationContent))
		if err != nil {
			logging.Logger.Errorf("failed to apply this '%s' migration file, error: ", err)
			return nil, fmt.Errorf("failed to run migrations")
		}

	}
	logging.Logger.Info("all migrations applied successfully")
	return db, nil
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

func getLastAppliedMigration(db *sql.DB, isFirstTime bool) (string, error) {
	if isFirstTime {
		migrationCreateSql := "CREATE TABLE IF NOT EXISTS `migration` (id int auto_increment primary key, migration_name varchar(255) not null unique, applied_at timestamp not null default current_timestamp);"
		_, err := db.Exec(migrationCreateSql)
		if err != nil {
			logging.Logger.Errorf("failed to create migration table for first time: %v", err)
			return "", fmt.Errorf("failed to create database")
		}
		return "001_migration.sql", nil
	}

	var lastMigration string
	err := db.QueryRow("SELECT migration_name FROM migration ORDER BY migration_name DESC LIMIT 1").Scan(&lastMigration)
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

	_, err = db.Exec(sqlContent)
	if err != nil {
		txn.Rollback()
		return fmt.Errorf("migration failed, rolled back: %w", err)
	}

	_, err = db.Exec("INSERT INTO migration (migration_name) VALUES (?)", name)
	if err != nil {
		txn.Rollback()
		return fmt.Errorf("failed to record migration, rolled back: %w", err)
	}

	if err = txn.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
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
					Code:       appErrors.ErrConflict,
					Message:    "The category already exists.",
					IsFeedBack: false,
				}
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to save expense category in Storage.SaveExpenseCategory() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
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
					Code:       appErrors.ErrConflict,
					Message:    "The category already exists.",
					IsFeedBack: false,
				}
			}
		}
		logging.Logger.Errorf("[TraceID=%s] | failed to save expense category in Storage.SaveExpenseCategory() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to check affected rows in Storage.UpdateSession() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	if rowsAffected == 0 {
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrAuth,
			Message:    "Session does not exist, please login.",
			IsFeedBack: false,
		}
	}

	return nil
}

func (mySql *MySQLStorage) GetSessionByToken(ctx context.Context, token string) (auth.Session, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	query := `SELECT id, token, created_at, expire_at, user_id FROM session WHERE token = ?`
	var dbSession dbSession
	err := mySql.db.QueryRow(query, token).Scan(
		&dbSession.ID,
		&dbSession.Token,
		&dbSession.CreatedAt,
		&dbSession.ExpireAt,
		&dbSession.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Session{}, appErrors.ErrorResponse{
				Code:       appErrors.ErrAuth,
				Message:    "Session does not exist, please login.",
				IsFeedBack: false,
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to get session by token in Storage.GetSessionByToken() function | Error: %v", traceID, err)
		return auth.Session{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	createdAt, err := time.Parse("2006-01-02 15:04:05", dbSession.CreatedAt)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at row in Storage.GetSessionByToken() function | Error: %v", traceID, err)
		return auth.Session{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}
	expireAt, err := time.Parse("2006-01-02 15:04:05", dbSession.ExpireAt)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse updated_at row in Storage.GetSessionByToken() function | Error: %v", traceID, err)
		return auth.Session{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	session := auth.Session{
		ID:        dbSession.ID,
		Token:     dbSession.Token,
		CreatedAt: createdAt,
		ExpireAt:  expireAt,
		UserID:    dbSession.UserID,
	}

	return session, nil
}

func (mySql *MySQLStorage) CheckSession(ctx context.Context, token string) (string, error) {
	query := `SELECT user_id, expire_at FROM session WHERE token = ?`

	var userID string
	var expireAtString string
	traceID := contextutil.TraceIDFromContext(ctx)

	err := mySql.db.QueryRow(query, token).Scan(&userID, &expireAtString)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", appErrors.ErrorResponse{
				Code:       appErrors.ErrAuth,
				Message:    "Session does not exist, please login.",
				IsFeedBack: false,
			}
		}
		logging.Logger.Errorf("[TraceID=%s] | failed to check session existance in Storage.CheckSession() function | Error: %v", traceID, err)
		return "", appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	expireAt, err := time.Parse("2006-01-02 15:04:05", expireAtString)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at row in Storage.CheckSession() function | Error: %v", traceID, err)
		return "", appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	now := time.Now().UTC()
	if expireAt.Before(now) {
		return "", appErrors.ErrorResponse{
			Code:       appErrors.ErrAuth,
			Message:    "Your session expired, please login again.",
			IsFeedBack: false,
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
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
				IsFeedBack: true,
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
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		expenseCategoryName = strings.ToLower(expenseCategoryName)
		if expenseCategoryName != "" && expenseCategoryName == strings.ToLower(categoryName) {
			return true, "-", nil
		}
	}
	return false, "", appErrors.ErrorResponse{
		Code:       appErrors.ErrInvalidInput,
		Message:    "Invalid category type",
		IsFeedBack: false,
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
					Code:       appErrors.ErrInternal,
					Message:    fmt.Sprintf("Please report this issue using the following ID: [%s]", traceID),
					IsFeedBack: true,
				}
			}

			return nil
		} else {
			return appErrors.ErrorResponse{
				Code:       appErrors.ErrInvalidInput,
				Message:    "Invalid category type for transaction",
				IsFeedBack: false,
			}
		}
	}
	return appErrors.ErrorResponse{
		Code:       appErrors.ErrInvalidInput,
		Message:    "The category does not exist, please create the category",
		IsFeedBack: false,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
		var createdAtStr string
		var updatedAtStr string

		err := rows.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAtStr, &updatedAtStr, &category.Note, &category.CreatedBy)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.processIncomeRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at in Storage.processIncomeRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtStr)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to parse updated_at in Storage.processIncomeRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return categories, nil
}

func (mySql *MySQLStorage) GetFilteredIncomeCategories(ctx context.Context, userID string, filters *budget.IncomeCategoryList) ([]budget.IncomeCategoryResponse, error) {
	query := "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_category WHERE created_by = ?"
	args := []interface{}{userID}
	traceID := contextutil.TraceIDFromContext(ctx)

	// Without filters

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)

		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to get all income categories in Storage.GetFilteredIncomeCategories() function | Error: %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}
		categories, err := mySql.processIncomeRows(ctx, rows, userID)

		if err != nil {
			return nil, err
		}
		return categories, nil
	}

	// With filters

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
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered categories in Storage.GetFilteredIncomeCategories() function | Error: %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
		var createdAtStr string
		var updatedAtStr string

		err := rows.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &createdAtStr, &updatedAtStr, &category.Note, &category.CreatedBy)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.processExpenseRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at in Storage.processExpenseRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtStr)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to parse updated_at in Storage.processExpenseRows() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return categories, nil
}

func (mySql *MySQLStorage) GetFilteredExpenseCategories(ctx context.Context, userID string, filters *budget.ExpenseCategoryList) ([]budget.ExpenseCategoryResponse, error) {
	query := "SELECT id, name, max_amount, period_day, created_at, updated_at, note, created_by FROM expense_category WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to get all expense categories in Storage.GetFilteredExpenseCategories() function | Error : %v", specialErrId, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", specialErrId),
				IsFeedBack: true,
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

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to get filtered expense categories in Storage.GetFilteredExpenseCategories() function | Error : %v", specialErrId, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", specialErrId),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	query = "SELECT id, name, max_amount, period_day, created_at, updated_at, note, created_by FROM expense_category WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, filters.ID)

	var category budget.ExpenseCategoryResponse
	var createdAt string
	var updatedAt string

	err = row.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.UpdateExpenseCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at field in Storage.UpdateExpenseCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}
	category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse updated_at field in Storage.UpdateExpenseCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	query = "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_category WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, filters.ID)

	var category budget.IncomeCategoryResponse
	var createdAt string
	var updatedAt string

	err = row.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.UpdateIncomeCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at field in Storage.UpdateIncomeCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to parse updated_at field in Storage.UpdateIncomeCategory() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	deleteCategoryQuery := "DELETE FROM expense_category WHERE created_by = ? AND id = ?;"
	result, err := tx.Exec(deleteCategoryQuery, userId, categoryId)
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] |  failed to delete expense category in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to check expense category delete status in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}

	}
	if rowsAffected == 0 {
		tx.Rollback()
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    "The category does not exist.",
			IsFeedBack: false,
		}
	}

	if err := tx.Commit(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to commit  SQL transaction in Storage.DeleteExpenseCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return nil
}

func (mySql *MySQLStorage) getCategoryNameById(traceID string, userID string, categoryId string, categoryType string) (*string, error) {
	var query string

	switch categoryType {
	case "-":
		query = "SELECT name FROM expense_categories WHERE created_by = ? AND id = ?;"
	case "+":
		query = "SELECT name FROM income_categories WHERE created_by = ? AND id = ?;"
	default:
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInvalidInput,
			Message:    "Invalid category type.",
			IsFeedBack: false,
		}
	}

	row := mySql.db.QueryRow(query, userID, categoryId)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInvalidInput,
				Message:    "The category does not exists.",
				IsFeedBack: false,
			}
		}
		logging.Logger.Errorf("[TraceID=%s] | failed to get category name by id from Storage.getCategoryNameById() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	deleteCategoryQuery := "DELETE FROM income_category WHERE created_by = ? AND id = ?;"
	result, err := tx.Exec(deleteCategoryQuery, userId, categoryId)
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to delete income category in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to check income category delete status in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	if rowsAffected == 0 {
		tx.Rollback()
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInvalidInput,
			Message:    "The category does not exist.",
			IsFeedBack: false,
		}
	}

	if err := tx.Commit(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] |  failed to commit SQL transaction  in Storage.DeleteIncomeCategory() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
		var createdAt string

		err := rows.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &createdAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.processTransactionRows() | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to parse created_at in Storage.processTransactionRows() | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
				IsFeedBack: true,
			}
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to iterate rows in Storage.processTransactionRows() | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return transactions, nil
}

func (mySql *MySQLStorage) GetFilteredTransactions(ctx context.Context, userID string, filters *budget.TransactionList) ([]budget.Transaction, error) {
	traceID := contextutil.TraceIDFromContext(ctx)
	query := "SELECT id, category_name, amount, currency, created_at, note, created_by, category_type FROM transaction WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | failed to get all transactions from Storage.GetFilteredTransactions() function | Error : %v", traceID, err)
			return nil, appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
				IsFeedBack: true,
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

	rows, err := mySql.db.Query(query, args...)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered transactions from Storage.GetFilteredTransactions() function | Error : %v", traceID, err)
		return nil, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
	var createdAt string
	err := row.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &createdAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return budget.Transaction{}, appErrors.ErrorResponse{
				Code:       appErrors.ErrInvalidInput,
				Message:    "The category does not exist.",
				IsFeedBack: false,
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to scan row in Storage.GetTransactionById() function | Error : %v", traceID, err)
		return budget.Transaction{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		logging.Logger.Errorf("special_id: %s | failed to parse created_at field in Storage.GetTransactionById() function | Error : %v", traceID, err)
		return budget.Transaction{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return transaction, nil
}

func (mySql *MySQLStorage) ValidateUser(ctx context.Context, credentials auth.UserCredentialsPure) (auth.User, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	logging.Logger.Infof("username-storage: %s", credentials.UserName)
	logging.Logger.Infof("password-storage: %s", credentials.PasswordPlain)

	query := "SELECT id, username, fullname, hashed_password, email FROM user WHERE username = ?;"
	row := mySql.db.QueryRow(query, credentials.UserName)
	var user auth.User
	err := row.Scan(&user.ID, &user.UserName, &user.FullName, &user.PasswordHashed, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, appErrors.ErrorResponse{
				Code:       appErrors.ErrAuth,
				Message:    "The user does not exist, please sign up.",
				IsFeedBack: false,
			}
		}

		logging.Logger.Errorf("[TraceID=%s] | failed to scan user row in Storage.ValidateUser() function | Error : %v", traceID, err)
		return auth.User{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}
	if auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) != true {
		return auth.User{}, appErrors.ErrorResponse{
			Code:       appErrors.ErrInvalidInput,
			Message:    "The password is wrong",
			IsFeedBack: false,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
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
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return nil
}

func (mySql *MySQLStorage) DeleteUser(ctx context.Context, userId string, deleteReq auth.DeleteUser) error {
	traceID := contextutil.TraceIDFromContext(ctx)

	txn, err := mySql.db.Begin()
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to start SQL transaction in Storage.DeleteUser() function | Error: %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	var hashedPassword string
	passwordQuery := "SELECT hashed_password FROM user WHERE id = ?;"
	row := mySql.db.QueryRow(passwordQuery, userId)

	if err := row.Scan(&hashedPassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.ErrorResponse{
				Code:       appErrors.ErrInternal,
				Message:    "User does not exist.",
				IsFeedBack: false,
			}
		}

		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to scan user row in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	if auth.ComparePasswords(hashedPassword, deleteReq.Password) != true {
		txn.Rollback()
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInvalidInput,
			Message:    "Password is wrong",
			IsFeedBack: false,
		}
	}

	sessionDelQuery := "DELETE FROM session WHERE user_id = ?;"
	_, err = mySql.db.Exec(sessionDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s]| failed to delete all user sessions in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	txnDelQuery := "DELETE FROM transaction WHERE created_by = ?;"
	_, err = mySql.db.Exec(txnDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to delete all user transactions in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	incomeDelQuery := "DELETE FROM income_category WHERE created_by = ?;"
	_, err = mySql.db.Exec(incomeDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to delete all user income categories in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	expenseDelQuery := "DELETE FROM expense_category WHERE created_by = ?;"
	_, err = mySql.db.Exec(expenseDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s]| failed to delete all user expense categories in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	userDelQuery := "DELETE FROM user where id = ?;"
	_, err = mySql.db.Exec(userDelQuery, userId)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s]| failed to delete user in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	deleteInfoQuery := "INSERT INTO deleted_account (reason) VALUES (?);"
	_, err = mySql.db.Exec(deleteInfoQuery, deleteReq.Reason)
	if err != nil {
		txn.Rollback()
		logging.Logger.Errorf("[TraceID=%s] | failed to insert delete info in Storage.DeleteUser() function | Error : %v", traceID, err)
		return appErrors.ErrorResponse{
			Code:       appErrors.ErrInternal,
			Message:    fmt.Sprintf("Please report this issue the  following ID: [%s]", traceID),
			IsFeedBack: true,
		}
	}

	return nil
}

func (mySql *MySQLStorage) GetStorageType() string {
	return "MySQL"
}
