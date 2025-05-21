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
)

var bt budget.BudgetTracker // Global

func main() {
	if err := logging.Init("debug"); err != nil {
		fmt.Println("failed to initalize logger: %w", err)
		return
	}
	logging.Logger.Info("Application started")

	db, err := storage.Init()
	if err != nil {
		logging.Logger.Errorf("failed to initalize database: %v", err)
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

	server.HandleFunc("POST /register", iz.Bind(api.SaveUserHandler))
	server.HandleFunc("POST /login", iz.Bind(api.LoginUserHandler))
	server.HandleFunc("POST /logout", iz.Bind(api.LogoutUserHandler))
	server.HandleFunc("POST /transaction", iz.Bind(api.SaveTransactionHandler))
	server.HandleFunc("POST /category/expense", iz.Bind(api.SaveExpenseCategoryHandler))
	server.HandleFunc("PUT /category/expense", iz.Bind(api.UpdateExpenseCategoryHandler))
	server.HandleFunc("DELETE /category/expense/{id}", iz.Bind(api.DeleteExpenseCategoryHandler))
	server.HandleFunc("GET /category/expense", iz.Bind(api.GetFilteredExpenseCategoriesHandler))
	server.HandleFunc("POST /category/income", iz.Bind(api.SaveIncomeCategoryHandler))
	server.HandleFunc("GET /category/income", iz.Bind(api.GetFilteredIncomeCategoriesHandler))
	server.HandleFunc("GET /transaction", iz.Bind(api.GetFilteredTransactionsHandler))
	server.HandleFunc("GET /transaction/{id}", iz.Bind(api.GetTransactionByIdHandler))

	fmt.Println("server is running")
	port := "8080"
	err = http.ListenAndServe(":"+port, server)
	if err != nil {
		logging.Logger.Errorf("failed to start server: %v", err)
		return
	}
}
