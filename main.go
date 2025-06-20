package main

import (
	"fmt"
	"net/http"

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
	AllowedOrigins:   []string{"*"}, // change with actual address in production
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowedHeaders:   []string{"Authorization", "Content-Type"},
	AllowCredentials: true,
})

func main() {
	if err := logging.Init("debug"); err != nil {
		fmt.Println("failed to initalize logger: %w", err)
		return
	}
	logging.Logger.Info("application starting")

	db, err := storage.Init()
	if err != nil {
		logging.Logger.Errorf("failed to initalize database: %v", err)
		fmt.Println("failed to initalize database")
		return
	}

	storageInstance := storage.NewMySQLStorage(db)
	if storageInstance == nil {
		logging.Logger.Errorf("failed to create instance of database: %v", err)
		fmt.Println("failed to create storage instance")
		return
	}

	bt = budget.NewBudgetTracker(storageInstance)

	server := http.NewServeMux()
	api := api.NewApi(&bt)

	// USER ENDPOINT START.
	server.HandleFunc("POST /register", iz.Bind(api.SaveUserHandler))         // Create User
	server.HandleFunc("POST /login", iz.Bind(api.LoginUserHandler))           // Login User
	server.HandleFunc("GET /logout", iz.Bind(api.LogoutUserHandler))          // Logout User
	server.HandleFunc("POST /remove-account", iz.Bind(api.DeleteUserHandler)) // Get User by token
	// USER ENDPOINT END.

	// TRANSACTION ENDPOINT START.
	server.HandleFunc("POST /transaction", iz.Bind(api.SaveTransactionHandler))        // Create Transaction
	server.HandleFunc("GET /transaction", iz.Bind(api.GetFilteredTransactionsHandler)) // Get Transactions with filters
	server.HandleFunc("GET /transaction/{id}", iz.Bind(api.GetTransactionByIdHandler)) // Get transation by ID
	server.Handle("POST /image-process", iz.Bind(api.ProcessImageHandler))             // Take image from user, and returns possible transaction fields
	server.Handle("OPTIONS /image-process", corsConf.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	// TRANSACTION ENDPOINT END.

	// EXPENSE CATEGORY ENDPOINT START.
	server.HandleFunc("POST /category/expense", iz.Bind(api.SaveExpenseCategoryHandler))          // Create Expense Category
	server.HandleFunc("GET /category/expense", iz.Bind(api.GetFilteredExpenseCategoriesHandler))  // Get Expense Categories with filters
	server.HandleFunc("PUT /category/expense", iz.Bind(api.UpdateExpenseCategoryHandler))         // Update Expense Category
	server.HandleFunc("DELETE /category/expense/{id}", iz.Bind(api.DeleteExpenseCategoryHandler)) // Delete Expense Category
	// EXPENSE CATEGORY ENDPOINT END.

	// INCOME CATEGORY ENDPOINT START.
	server.HandleFunc("POST /category/income", iz.Bind(api.SaveIncomeCategoryHandler))          // Create Income Category
	server.HandleFunc("GET /category/income", iz.Bind(api.GetFilteredIncomeCategoriesHandler))  // Get Income Categories with filters
	server.HandleFunc("PUT /category/income", iz.Bind(api.UpdateIncomeCategoryHandler))         // Update Income Category
	server.HandleFunc("DELETE /category/income/{id}", iz.Bind(api.DeleteIncomeCategoryHandler)) // Delete Income Category

	port := "8060"
	fmt.Println("Starting server on port", port)
	err = http.ListenAndServe(":"+port, server)
	if err != nil {
		logging.Logger.Errorf("failed to start server: %v", err)
		fmt.Println("failed to start server")
		return
	}
}
