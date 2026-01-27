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
	"github.com/rs/cors"
)

var bt budget.BudgetTracker // Global

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
	server.HandleFunc("POST /api/register", iz.Bind(api.SaveUserHandler))         // Create User
	server.HandleFunc("POST /api/login", iz.Bind(api.LoginUserHandler))           // Login User
	server.HandleFunc("GET /api/logout", iz.Bind(api.LogoutUserHandler))          // Logout User
	server.HandleFunc("POST /api/remove-account", iz.Bind(api.DeleteUserHandler)) // Remove User
	server.HandleFunc("GET /api/download-user-data", api.DownloadUserData)        // Download User Data
	server.HandleFunc("GET /api/check-token", iz.Bind(api.CheckToken))            // Check User Token
	server.HandleFunc("GET /api/account", iz.Bind(api.GetAccountInfo))            // Account Info

	// TRANSACTION ENDPOINTS.
	server.HandleFunc("POST /api/transaction", iz.Bind(api.SaveTransactionHandler))        // Create Transaction
	server.HandleFunc("GET /api/transaction", iz.Bind(api.GetFilteredTransactionsHandler)) // Get Transactions with filters
	server.HandleFunc("GET /api/transaction/{id}", iz.Bind(api.GetTransactionByIdHandler)) // Get Transation by ID
	server.Handle("POST /api/image-process", iz.Bind(api.ProcessImageHandler))             // Take image from user, and returns possible transaction fields

	// EXPENSE CATEGORY ENDPOINTS.
	server.HandleFunc("POST /api/category/expense", iz.Bind(api.SaveExpenseCategoryHandler))          // Create Expense Category
	server.HandleFunc("GET /api/category/expense", iz.Bind(api.GetFilteredExpenseCategoriesHandler))  // Get Expense Categories with filters
	server.HandleFunc("PUT /api/category/expense", iz.Bind(api.UpdateExpenseCategoryHandler))         // Update Expense Category
	server.HandleFunc("DELETE /api/category/expense/{id}", iz.Bind(api.DeleteExpenseCategoryHandler)) // Delete Expense Category

	// INCOME CATEGORY ENDPOINTS.
	server.HandleFunc("POST /api/category/income", iz.Bind(api.SaveIncomeCategoryHandler))          // Create Income Category
	server.HandleFunc("GET /api/category/income", iz.Bind(api.GetFilteredIncomeCategoriesHandler))  // Get Income Categories with filters
	server.HandleFunc("PUT /api/category/income", iz.Bind(api.UpdateIncomeCategoryHandler))         // Update Income Category
	server.HandleFunc("DELETE /api/category/income/{id}", iz.Bind(api.DeleteIncomeCategoryHandler)) // Delete Income Category

	// STATISTICS ENDPOINTS.
	server.HandleFunc("GET /api/statistics/expense", iz.Bind(api.GetExpenseCategoryStatsHandler)) // Get Statistics of expense categories
	server.HandleFunc("GET /api/statistics/income", iz.Bind(api.GetIncomeCategoryStatsHandler))   // Get Statistics of income categories
	server.HandleFunc("GET /api/statistics/transaction", iz.Bind(api.GetTransactionStatsHandler)) // Get Statistics of transactions

	port := os.Getenv("APP_PORT")
	if port == "" {
		logging.Logger.Info("APP_PORT environment variable not set, using default port 8060")
		port = "8080"
	}
	fmt.Println("Starting server on port: ", port)
	handlerwithCors := corsConf.Handler(server)
	err = http.ListenAndServe(":"+port, handlerwithCors) // Start the server
	if err != nil {
		logging.Logger.Errorf("failed to start server: %v", err)
		return
	}
}
