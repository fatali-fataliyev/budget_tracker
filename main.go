package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/0xcafe-io/iz"
	"github.com/fatali-fataliyev/budget_tracker/api"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/internal/storage"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/cors"
)

var bt budget.BudgetTracker // Global
type contextKey string

var traceIDKey contextKey = "traceID"

var corsConf = cors.New(cors.Options{
	AllowedOrigins:   []string{"*"},
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowedHeaders:   []string{"Authorization", "Content-Type"},
	AllowCredentials: true,
})

func main() {
	if err := logging.Init("debug"); err != nil {
		fmt.Println("failed to initialize logger: %w", err)
		return
	}
	logging.Logger.Info("application starting...")

	db, err := storage.Init()
	if err != nil {
		logging.Logger.Errorf("failed to initialize database: %v", err)
		return
	}

	storageInstance := storage.NewMySQLStorage(db)
	if storageInstance == nil {
		logging.Logger.Errorf("failed to create instance of database: %v", err)
		return
	}

	bt = budget.NewBudgetTracker(storageInstance)

	server := http.NewServeMux()
	api := api.NewApi(&bt)

	// USER ENDPOINTS.
	server.HandleFunc("POST /register", iz.Bind(api.SaveUserHandler))         // Create User
	server.HandleFunc("POST /login", iz.Bind(api.LoginUserHandler))           // Login User
	server.HandleFunc("GET /logout", iz.Bind(api.LogoutUserHandler))          // Logout User
	server.HandleFunc("POST /remove-account", iz.Bind(api.DeleteUserHandler)) // Remove User
	server.HandleFunc("GET /download-user-data", api.DownloadUserData)        // Download User Data
	server.HandleFunc("GET /check-token", iz.Bind(api.CheckToken))            // Check User Token
	server.HandleFunc("GET /account", iz.Bind(api.GetAccountInfo))            // Account Info

	// TRANSACTION ENDPOINTS.
	server.HandleFunc("POST /transaction", iz.Bind(api.SaveTransactionHandler))        // Create Transaction
	server.HandleFunc("GET /transaction", iz.Bind(api.GetFilteredTransactionsHandler)) // Get Transactions with filters
	server.HandleFunc("GET /transaction/{id}", iz.Bind(api.GetTransactionByIdHandler)) // Get Transation by ID
	server.Handle("POST /image-process", iz.Bind(api.ProcessImageHandler))             // Take image from user, and returns possible transaction fields

	// EXPENSE CATEGORY ENDPOINTS.
	server.HandleFunc("POST /category/expense", iz.Bind(api.SaveExpenseCategoryHandler))          // Create Expense Category
	server.HandleFunc("GET /category/expense", iz.Bind(api.GetFilteredExpenseCategoriesHandler))  // Get Expense Categories with filters
	server.HandleFunc("PUT /category/expense", iz.Bind(api.UpdateExpenseCategoryHandler))         // Update Expense Category
	server.HandleFunc("DELETE /category/expense/{id}", iz.Bind(api.DeleteExpenseCategoryHandler)) // Delete Expense Category

	// INCOME CATEGORY ENDPOINTS.
	server.HandleFunc("POST /category/income", iz.Bind(api.SaveIncomeCategoryHandler))          // Create Income Category
	server.HandleFunc("GET /category/income", iz.Bind(api.GetFilteredIncomeCategoriesHandler))  // Get Income Categories with filters
	server.HandleFunc("PUT /category/income", iz.Bind(api.UpdateIncomeCategoryHandler))         // Update Income Category
	server.HandleFunc("DELETE /category/income/{id}", iz.Bind(api.DeleteIncomeCategoryHandler)) // Delete Income Category

	// STATISTICS ENDPOINTS.
	server.HandleFunc("GET /statistics/expense", iz.Bind(api.GetExpenseCategoryStatsHandler)) // Get Statistics of expense categories
	server.HandleFunc("GET /statistics/income", iz.Bind(api.GetIncomeCategoryStatsHandler))   // Get Statistics of income categories
	server.HandleFunc("GET /statistics/transaction", iz.Bind(api.GetTransactionStatsHandler)) // Get Statistics of transactions

	port := os.Getenv("APP_PORT")
	if port == "" {
		logging.Logger.Info("APP_PORT environment variable not set, using default port 8060")
		port = "8060"
	}
	fmt.Println("Starting server on port: ", port)
	handlerwithCors := corsConf.Handler(server)
	err = http.ListenAndServe(":"+port, handlerwithCors) // Start the server
	if err != nil {
		logging.Logger.Errorf("failed to start server: %v", err)
		return
	}
}
