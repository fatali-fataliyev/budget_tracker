package storage

import (
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
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
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
		logging.Logger.Infof("Database %s does not exist, creating...", dbname)

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

	dbTimezoneSql := fmt.Sprintf("SET GLOBAL time_zone = '+00:00'")
	_, err = rootDb.Exec(dbTimezoneSql)
	if err != nil {
		logging.Logger.Errorf("failed to set timezone: %v", err)
		return nil, fmt.Errorf("failed to create database")
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

	for {
		if err := db.Ping(); err == nil {
			break
		}
		logging.Logger.Warnf("waiting for database...")
		time.Sleep(3 * time.Second)
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
		migrationCreateSql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `migration` (id int auto_increment primary key, migration_name varchar(255) not null unique, applied_at timestamp not null default current_timestamp);")
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

func (mySql *MySQLStorage) SaveUser(user auth.User) error {
	query := "INSERT INTO users (id, username, fullname, hashed_password, email, pending_email) VALUES (?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, user.ID, user.UserName, user.FullName, user.PasswordHashed, user.Email, user.Email)

	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to save user from SaveUser() function, error: %v", specialErrId, err)
		return fmt.Errorf("please send feedback with this ID: %s", specialErrId)
	}
	return nil
}

func (mySql *MySQLStorage) SaveSession(session auth.Session) error {
	query := "INSERT INTO sessions (id, token, created_at, expire_at, user_id) VALUES (?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, session.ID, session.Token, session.CreatedAt, session.ExpireAt, session.UserID)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to save session from SaveSession() function, error: %v", specialErrId, err)
		return fmt.Errorf("failed to save session, please send feedback by this ID: %s", specialErrId)
	}
	return nil
}
func (mySql *MySQLStorage) SaveExpenseCategory(category budget.ExpenseCategory) error {
	query := "INSERT INTO expense_categories (id, name, max_amount, period_day, created_at, updated_at, note, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, category.ID, category.Name, category.MaxAmount, category.PeriodDay, category.CreatedAt, category.UpdatedAt, category.Note, category.CreatedBy)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 {
				return fmt.Errorf("%w: this category already exists.", appErrors.ErrConflict)
			}
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to save expense category from SaveExpenseCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("failed to save expense category, please send feedback by this ID: %s", specialErrId)
	}
	return nil
}

func (mySql *MySQLStorage) SaveIncomeCategory(category budget.IncomeCategory) error {
	query := "INSERT INTO income_categories (id, name, target_amount, created_at, updated_at, note, created_by) VALUES (?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, category.ID, category.Name, category.TargetAmount, category.CreatedAt, category.UpdatedAt, category.Note, category.CreatedBy)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 {
				return fmt.Errorf("%w: this category already exists.", appErrors.ErrConflict)
			}
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to save income category from SaveIncomeCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("failed to save income category, please send feedback by this ID: %s", specialErrId)
	}
	return nil
}

func (mySql *MySQLStorage) UpdateSession(userId string, newExpireDate time.Time) error {
	query := `UPDATE sessions SET expire_at = ? WHERE user_id = ?`

	res, err := mySql.db.Exec(query, newExpireDate, userId)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to extend session expire date from UpdateSession() function, error : %v", specialErrId, err)
		return fmt.Errorf("session expire date exten failed, please send feedback by this ID: %s", specialErrId)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to check extended session status from UpdateSession() function, error : %v", specialErrId, err)
		return fmt.Errorf("failed to check session expire status, please send feedback by this ID: %s", specialErrId)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: session does not exist, please login", appErrors.ErrNotFound)
	}

	return nil
}

func (mySql *MySQLStorage) GetSessionByToken(token string) (auth.Session, error) {
	query := `SELECT id, token, created_at, expire_at, user_id FROM sessions WHERE token = ?`

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
			return auth.Session{}, fmt.Errorf("%w: session does not exist, please login", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan row from GetSessionByToken() function, error: %v", specialErrId, err)
		return auth.Session{}, fmt.Errorf("failed to get session by token, please send feedback by this ID: %s", specialErrId)
	}

	createdAt, err := time.Parse("2006-01-02 15:04:05", dbSession.CreatedAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse created_at field from GetSessionByToken() function, error : %v", specialErrId, err)
		return auth.Session{}, fmt.Errorf("failed to check session create date, please send feedback by this ID: %s", specialErrId)
	}
	expireAt, err := time.Parse("2006-01-02 15:04:05", dbSession.ExpireAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse expire_at field from GetSessionByToken() function, error : %v", specialErrId, err)
		return auth.Session{}, fmt.Errorf("failed to check session expire date, please send feedback by this ID: %s", specialErrId)
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

func (mySql *MySQLStorage) CheckSession(token string) (string, error) {
	query := `SELECT user_id, expire_at FROM sessions WHERE token = ?`

	var userID string
	var expireAtString string

	err := mySql.db.QueryRow(query, token).Scan(&userID, &expireAtString)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: session does not exist, please login", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to check session from CheckSession() function, error : %v", specialErrId, err)
		return "", fmt.Errorf("failed to check session, please send feedback by this ID: %s", specialErrId)
	}

	expireAt, err := time.Parse("2006-01-02 15:04:05", expireAtString)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse expire_at field from CheckSession() function, error : %v", specialErrId, err)
		return "", fmt.Errorf("failed to check session expire date, please send feedback by this ID: %s", specialErrId)
	}

	now := time.Now()

	if expireAt.Before(now) {
		return "", fmt.Errorf("%w: session expired, please login again", appErrors.ErrAuth)
	}

	return userID, nil
}

func (mySql *MySQLStorage) SaveTransaction(t budget.Transaction) error {
	query := "INSERT INTO transactions (id, category_name, amount, currency, created_at, note, created_by, category_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
	_, err := mySql.db.Exec(query, t.ID, t.CategoryName, t.Amount, t.Currency, t.CreatedAt, t.Note, t.CreatedBy, t.CategoryType)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to save transaction from SaveTransaction() function, error : %v", specialErrId, err)
		return fmt.Errorf("failed to save transaction, please send feedback by this ID: %s", specialErrId)
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
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to get total amount from GetTotalAmountOfTransactions() function, error : %v", specialErrId, err)
		return 0, fmt.Errorf("failed to get total amount of transactions, please send feedback by this ID: %s", specialErrId)
	}
	return total, nil
}

func (mySql *MySQLStorage) GetFilteredIncomeCategories(userID string, filters *budget.IncomeCategoryList) ([]budget.IncomeCategoryResponse, error) {
	query := "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_categories WHERE created_by = ?"
	args := []interface{}{userID}

	// Without filters

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)

		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to get all income categories from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		defer rows.Close()

		var categories []budget.IncomeCategoryResponse

		for rows.Next() {
			var category budget.IncomeCategoryResponse
			var createdAt string
			var updatedAt string

			err = rows.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to scan row from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}
			category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to parse created at row from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}
			category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to parse updated at row from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}

			categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "+")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			category.Amount = categoryAmount
			categories = append(categories, category)
		}
		if err := rows.Err(); err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to iterate over rows from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
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
		return nil, err
	}

	defer rows.Close()

	var categories []budget.IncomeCategoryResponse
	for rows.Next() {
		var category budget.IncomeCategoryResponse
		var createdAt string
		var updatedAt string

		err = rows.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to scan row from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to parse created at row from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to parse updated at row from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}

		categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "+")
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		category.Amount = categoryAmount
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to iterate over rows from GetFilteredIncomeCategories() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
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
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to get all expense categories from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}

		defer rows.Close()

		var categories []budget.ExpenseCategoryResponse
		for rows.Next() {
			var category budget.ExpenseCategoryResponse
			var createdAt string
			var updatedAt string

			err = rows.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to scan row from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}
			category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to parse created at date from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}
			category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to parse updated at date from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}

			categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "-")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			category.Amount = categoryAmount

			categories = append(categories, category)
		}
		if err := rows.Err(); err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to iterate over rows from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
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
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to scan row from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to parse created at date from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to parse updated at date from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}

		categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "-")
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		category.Amount = categoryAmount

		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to iterate over rows from GetFilteredExpenseCategories() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	return categories, nil
}

func (mySql *MySQLStorage) UpdateExpenseCategory(userID string, filters budget.UpdateExpenseCategoryRequest) (*budget.ExpenseCategoryResponse, error) {
	query := "UPDATE expense_categories SET name = ?, max_amount = ?, period_day = ?, updated_at = ?, note = ? WHERE created_by = ? AND id = ?;"
	_, err := mySql.db.Exec(query, filters.NewName, filters.NewMaxAmount, filters.NewPeriodDay, filters.UpdateTime, filters.NewNote, userID, filters.ID)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to update expense category from UpdateExpenseCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	query = "SELECT id, name, max_amount, period_day, created_at, updated_at, note, created_by FROM expense_categories WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, filters.ID)

	var category budget.ExpenseCategoryResponse
	var createdAt string
	var updatedAt string

	err = row.Scan(&category.ID, &category.Name, &category.MaxAmount, &category.PeriodDay, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: category does not exist", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan row from UpdateExpenseCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse create at time from UpdateExpenseCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}
	category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse update at time from UpdateExpenseCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "-")
	if err != nil {
		return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
	}
	category.Amount = categoryAmount

	return &category, nil
}

func (mySql *MySQLStorage) UpdateIncomeCategory(userID string, filters budget.UpdateIncomeCategoryRequest) (*budget.IncomeCategoryResponse, error) {
	query := "UPDATE income_categories SET name = ?, target_amount = ?, updated_at = ?, note = ? WHERE created_by = ? AND id = ?;"
	_, err := mySql.db.Exec(query, filters.NewName, filters.NewTargetAmount, filters.UpdateTime, filters.NewNote, userID, filters.ID)

	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to update income category from UpdateIncomeCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	query = "SELECT id, name, target_amount, created_at, updated_at, note, created_by FROM income_categories WHERE created_by = ? AND id = ?;"
	row := mySql.db.QueryRow(query, userID, filters.ID)

	var category budget.IncomeCategoryResponse
	var createdAt string
	var updatedAt string

	err = row.Scan(&category.ID, &category.Name, &category.TargetAmount, &createdAt, &updatedAt, &category.Note, &category.CreatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: category not found", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan row from UpdateIncomeCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	category.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse create at time from UpdateIncomeCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}
	category.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse update at time from UpdateIncomeCategory() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	categoryAmount, err := mySql.GetTotalAmountOfTransactions(userID, category.Name, "+")
	if err != nil {
		return nil, fmt.Errorf("failed to get total amount of transactions: %w", err)
	}

	category.Amount = categoryAmount

	return &category, nil
}

func (mySql *MySQLStorage) DeleteExpenseCategory(userId string, categoryId string) error {
	tx, err := mySql.db.Begin()
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to start SQL transaction from DeleteExpenseCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	categoryName, err := mySql.getCategoryNameById(userId, categoryId, "-")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to get category name: %w", err)
	}

	deleteTxQuery := "DELETE FROM transactions WHERE created_by = ? AND category_name = ? AND category_type = '-';"
	_, err = tx.Exec(deleteTxQuery, userId, categoryName)
	if err != nil {
		tx.Rollback()
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to delete all related transactions from DeleteExpenseCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	deleteCategoryQuery := "DELETE FROM expense_categories WHERE created_by = ? AND id = ?;"
	result, err := tx.Exec(deleteCategoryQuery, userId, categoryId)
	if err != nil {
		tx.Rollback()
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to delete expense category from DeleteExpenseCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to check expense category delete status from DeleteExpenseCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}
	if rowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("%w: category does not exist.", appErrors.ErrNotFound)
	}

	if err := tx.Commit(); err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to commit SQL transaction from DeleteExpenseCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	return nil
}

func (mySql *MySQLStorage) getCategoryNameById(userID string, categoryId string, categoryType string) (*string, error) {
	var query string

	if categoryType == "-" {
		query = "SELECT name FROM expense_categories WHERE created_by = ? AND id = ?;"
	} else if categoryType == "+" {
		query = "SELECT name FROM income_categories WHERE created_by = ? AND id = ?;"
	} else {
		return nil, fmt.Errorf("%w: unknown category type: %s", appErrors.ErrInvalidInput, categoryType)
	}
	row := mySql.db.QueryRow(query, userID, categoryId)

	var name string
	err := row.Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: category does not exist.", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to get category name by id from getCategoryNameById() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	return &name, nil
}

func (mySql *MySQLStorage) DeleteIncomeCategory(userId string, categoryId string) error {
	tx, err := mySql.db.Begin()
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to start SQL transaction from DeleteIncomeCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	categoryName, err := mySql.getCategoryNameById(userId, categoryId, "+")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to get category name: %w", err)
	}

	deleteTxQuery := "DELETE FROM transactions WHERE created_by = ? AND category_name = ? AND category_type = '+';"
	_, err = tx.Exec(deleteTxQuery, userId, categoryName)
	if err != nil {
		tx.Rollback()
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to delete all related transactions from DeleteIncomeCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	deleteCategoryQuery := "DELETE FROM income_categories WHERE created_by = ? AND id = ?;"
	result, err := tx.Exec(deleteCategoryQuery, userId, categoryId)
	if err != nil {
		tx.Rollback()
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to delete income category from DeleteIncomeCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to check income category delete status from DeleteIncomeCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}
	if rowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("%w: category does not exist.", appErrors.ErrNotFound)
	}

	if err := tx.Commit(); err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to commit SQL transaction from DeleteIncomeCategory() function, error : %v", specialErrId, err)
		return fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	return nil
}

func (mySql *MySQLStorage) GetFilteredTransactions(userID string, filters *budget.TransactionList) ([]budget.Transaction, error) {
	query := "SELECT id, category_name, amount, currency, created_at, note, created_by, category_type FROM transactions WHERE created_by = ?"
	args := []interface{}{userID}

	if filters.IsAllNil {
		query += ";"
		rows, err := mySql.db.Query(query, args...)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to get all transactions from GetFilteredTransactions() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}

		defer rows.Close()

		var transactions []budget.Transaction
		for rows.Next() {
			var transaction budget.Transaction
			var createdAt string

			err = rows.Scan(&transaction.ID, &transaction.CategoryName, &transaction.Amount, &transaction.Currency, &createdAt, &transaction.Note, &transaction.CreatedBy, &transaction.CategoryType)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to scan row from GetFilteredTransactions() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}
			transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				specialErrId := uuid.New().String()
				logging.Logger.Errorf("special_id: %s | failed to parse create at GetFilteredTransactions() function, error : %v", specialErrId, err)
				return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
			}
			transactions = append(transactions, transaction)
		}
		if err := rows.Err(); err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to iterate over rows from GetFilteredTransactions() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
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
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to get filtered transactions from GetFilteredTransactions() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			specialErrId := uuid.New().String()
			logging.Logger.Errorf("special_id: %s | failed to parse create at from GetFilteredTransactions() function, error : %v", specialErrId, err)
			return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to iterate over rows from GetFilteredTransactions() function, error : %v", specialErrId, err)
		return nil, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
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
			return budget.Transaction{}, fmt.Errorf("%w: transaction does not exist.", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan row from GetTransactionById() function, error : %v", specialErrId, err)
		return budget.Transaction{}, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
	}

	transaction.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to parse create at from GetTransactionById() function, error : %v", specialErrId, err)
		return budget.Transaction{}, fmt.Errorf("please send feedback by this ID: %s", specialErrId)
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
			return auth.User{}, fmt.Errorf("%w: user does not exist, register please", appErrors.ErrNotFound)
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan user row from ValidateUser(), error: %v", specialErrId, err)
		return auth.User{}, fmt.Errorf("failed to validate user, please send feedback with this ID: %s", specialErrId)
	}
	if auth.ComparePasswords(user.PasswordHashed, credentials.PasswordPlain) != true {
		return auth.User{}, fmt.Errorf("%w: password is wrong", appErrors.ErrInvalidInput)
	}
	return user, nil
}

func (mySql *MySQLStorage) IsUserExists(username string) (bool, error) {
	query := "SELECT 1 FROM users WHERE username = ?;"

	row := mySql.db.QueryRow(query, username)
	err := row.Scan()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan row from IsUserExists() function, error: %v", specialErrId, err)
		return false, fmt.Errorf("please send feedback with this ID: %s", specialErrId)
	}

	return true, nil
}

func (mySql *MySQLStorage) IsEmailConfirmed(emailAddress string) (bool, error) {
	query := "SELECT COUNT(*) FROM users WHERE email = ? AND pending_email IS NULL;"
	row := mySql.db.QueryRow(query, emailAddress)

	var count int
	err := row.Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to scan row from IsEmailConfirmed() function, error: %v", specialErrId, err)
		return false, fmt.Errorf("please send feedback with this ID: %s", specialErrId)
	}

	return count > 0, nil
}

func (mySql *MySQLStorage) LogoutUser(userId string, token string) error {
	query := "UPDATE sessions SET expire_at = UTC_TIMESTAMP() - INTERVAL 1 SECOND WHERE user_id = ? AND token = ?"

	_, err := mySql.db.Exec(query, userId, token)
	if err != nil {
		specialErrId := uuid.New().String()
		logging.Logger.Errorf("special_id: %s | failed to logout user from LogoutUser() function, error: %v", specialErrId, err)
		return fmt.Errorf("please send feedback with this ID: %s", specialErrId)
	}
	return nil
}

func (mySql *MySQLStorage) GetStorageType() string {
	return "MySQL"
}
