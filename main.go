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

func main() {

	// CORS POLICY
	var corsConf = cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	var bt budget.BudgetTracker // Global

	// Logger
	if err := logging.Init("debug"); err != nil {
		fmt.Println("failed to initialize logger: %w", err)
		return
	}

	logging.Logger.Info("application starting...")

	// Storage
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
	server.HandleFunc("POST /api/register", iz.Bind(api.SaveUserHandler))                         // Create User [OPEN]
	server.HandleFunc("POST /api/login", iz.Bind(api.LoginUserHandler))                           // Login User  [OPEN]
	server.Handle("GET /api/logout", iz.Bind(api.LogoutUserHandler))                              // Logout User [PROTECTED]
	server.Handle("POST /api/remove-account", api.AuthMiddleware(iz.Bind(api.DeleteUserHandler))) // Remove User [PROTECTED]
	server.HandleFunc("GET /api/download-user-data", api.DownloadUserData)                        // Download Data [PROTECTED]
	server.Handle("GET /api/check-token", api.AuthMiddleware(iz.Bind(api.CheckToken)))            // Check User Token [PROTECTED]
	server.Handle("GET /api/account", api.AuthMiddleware(iz.Bind(api.GetAccountInfo)))            // Account Info     [PROTECTED]

	// TRANSACTION ENDPOINTS.
	server.Handle("POST /api/transaction", api.AuthMiddleware(iz.Bind(api.SaveTransactionHandler)))        // Create Transaction         [PROTECTED]
	server.Handle("GET /api/transaction", api.AuthMiddleware(iz.Bind(api.GetFilteredTransactionsHandler))) // Get Transactions by filter [PROTECTED]
	server.Handle("GET /api/transaction/{id}", api.AuthMiddleware(iz.Bind(api.GetTransactionByIdHandler))) // Get Transation by ID       [PROTECTED]
	server.Handle("POST /api/image-process", api.AuthMiddleware(iz.Bind(api.ProcessImageHandler)))         // Image to Transaction       [PROTECTED]

	// EXPENSE CATEGORY ENDPOINTS.
	server.Handle("POST /api/category/expense", api.AuthMiddleware(iz.Bind(api.SaveExpenseCategoryHandler)))          // Create Expense Category        [PROTECTED]
	server.Handle("GET /api/category/expense", api.AuthMiddleware(iz.Bind(api.GetFilteredExpenseCategoriesHandler)))  // Get Expense Category by filter [PROTECTED]
	server.Handle("PUT /api/category/expense", api.AuthMiddleware(iz.Bind(api.UpdateExpenseCategoryHandler)))         // Update Expense Category        [PROTECTED]
	server.Handle("DELETE /api/category/expense/{id}", api.AuthMiddleware(iz.Bind(api.DeleteExpenseCategoryHandler))) // Delete Expense Category        [PROTECTED]

	// INCOME CATEGORY ENDPOINTS.
	server.Handle("POST /api/category/income", api.AuthMiddleware(iz.Bind(api.SaveIncomeCategoryHandler)))          // Create Income Category 		 [PROTECTED]
	server.Handle("GET /api/category/income", api.AuthMiddleware(iz.Bind(api.GetFilteredIncomeCategoriesHandler)))  // Get Income Category by filter [PROTECTED]
	server.Handle("PUT /api/category/income", api.AuthMiddleware(iz.Bind(api.UpdateIncomeCategoryHandler)))         // Update Income Category 		 [PROTECTED]
	server.Handle("DELETE /api/category/income/{id}", api.AuthMiddleware(iz.Bind(api.DeleteIncomeCategoryHandler))) // Delete Income Category 		 [PROTECTED]

	// STATISTICS ENDPOINTS.
	server.Handle("GET /api/statistics/expense", api.AuthMiddleware(iz.Bind(api.GetExpenseCategoryStatsHandler))) // Get Statistics of expense categories [PROTECTED]
	server.Handle("GET /api/statistics/income", api.AuthMiddleware(iz.Bind(api.GetIncomeCategoryStatsHandler)))   // Get Statistics of income categories  [PROTECTED]
	server.Handle("GET /api/statistics/transaction", api.AuthMiddleware(iz.Bind(api.GetTransactionStatsHandler))) // Get Statistics of transactions 	  [PROTECTED]

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
