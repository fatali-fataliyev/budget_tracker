package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/0xcafe-io/iz"
	"github.com/fatali-fataliyev/budget_tracker/api"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/internal/storage"
	_ "github.com/go-sql-driver/mysql"
	"github.com/subosito/gotenv"
)

var bt budget.BudgetTracker // Global

func initDB() (*sql.DB, error) {
	err := gotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load env variables")
	}
	username := os.Getenv("DBUSER")
	password := os.Getenv("DBPASS")
	dbname := os.Getenv("DBNAME")

	fmt.Println("Initializing Database...")

	db, err := sql.Open("mysql", username+":"+password+"@tcp(localhost:3306)/"+dbname)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	for err := db.Ping(); err != nil; {
		time.Sleep(1 * time.Second)
	}
	fmt.Println("connected.")
	return db, nil
}

func main() {
	db, err := initDB()
	_ = db
	if err != nil {
		fmt.Println("failed to initialize database: ", err)
		return
	}
	storageInstance := storage.NewMySQLStorage(db)
	if storageInstance == nil {
		fmt.Println("failed to initialize storage")
		return
	}

	bt = budget.NewBudgetTracker(storageInstance)

	server := http.NewServeMux()
	api := api.NewApi(&bt)

	server.HandleFunc("POST /register", iz.Bind(api.SaveUserHandler))
	server.HandleFunc("POST /login", iz.Bind(api.LoginUserHandler))
	server.HandleFunc("POST /logout", iz.Bind(api.LogoutUserHandler))
	server.HandleFunc("POST /transaction", iz.Bind(api.SaveTransactionHandler))
	server.HandleFunc("GET /transaction", iz.Bind(api.GetFilteredTransactionsHandler))
	server.HandleFunc("GET /total", iz.Bind(api.GetTotalsByTypeAndCurrencyHandler))
	server.HandleFunc("GET /transaction/{id}", iz.Bind(api.GetTransactionByIdHandler))
	server.HandleFunc("PUT /transaction/{id}", iz.Bind(api.UpdateTransactionHandler))
	server.HandleFunc("DELETE /transaction/{id}", iz.Bind(api.DeleteTransactionHandler))

	port := "8080"
	fmt.Println("Listen: http://localhost:" + port)
	err = http.ListenAndServe(":"+port, server)
	if err != nil {
		fmt.Println("failed to start server: ", err)
		return
	}
}
