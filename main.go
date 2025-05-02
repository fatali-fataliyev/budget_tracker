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
	server.HandleFunc("POST /category", iz.Bind(api.SaveCategoryHandler))
	server.HandleFunc("GET /category", iz.Bind(api.GetFilteredCategoriesHandler))
	server.HandleFunc("GET /category/{name}", iz.Bind(api.GetCategoryByNameHandler))
	server.HandleFunc("GET /transaction", iz.Bind(api.GetFilteredTransactionsHandler))
	server.HandleFunc("GET /transaction/{id}", iz.Bind(api.GetTransactionByIdHandler))
	server.HandleFunc("PUT /transaction/{id}", iz.Bind(api.UpdateTransactionHandler))
	server.HandleFunc("DELETE /transaction/{id}", iz.Bind(api.DeleteTransactionHandler))

	fmt.Println("server is running")
	port := "8080"
	err = http.ListenAndServe(":"+port, server)
	if err != nil {
		logging.Logger.Errorf("failed to start server: %v", err)
		return
	}
}
