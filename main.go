package main

import (
	"fmt"
	"net/http"
)

var bt BudgetTracker // Global

func main() {
	db, lastID, err := initDB()
	_ = db
	if err != nil {
		fmt.Println("failed to initialize database: ", err)
		return
	}

	bt = NewBudgetTracker(lastID, NewMySQLStorage(db), "internal")

	inMemoryStorageSettings := AuthUser{
		Username:      "",
		PasswordPlain: "",
	}
	//Connect internal memory
	if bt.storageType == "" || bt.storageType == "internal" {
		fmt.Println("internal memory connecting. . .")
		var username string
		var password string
		print("username: ")
		fmt.Scan(&username)
		print("password: ")
		fmt.Scan(&password)

		inMemoryStorageSettings.Username = username
		inMemoryStorageSettings.PasswordPlain = password
		fmt.Println("access checking. . .")
	}

	bt = NewBudgetTracker(0, NewInternalMemeoryStorage(inMemoryStorageSettings), "internal")

	server := http.NewServeMux() //HTTP Router, multiplexer. | More secure. | init func in 3rd libraries.

	server.HandleFunc("POST /auth/register", CreateUserHandler)   // Create User
	server.HandleFunc("POST /transaction", AddTransactionHandler) // Add Transaction

	server.HandleFunc("GET /transactions", GetTransactionsHandler) // Get all user's transactions
	server.HandleFunc("GET /transactions/{id}", GetByIdHandler)    // Get user's specific transaction.
	server.HandleFunc("GET /total/", GetTotalHandler)              // Get user's total expenses or incomes.

	server.HandleFunc("PUT /transactions/{id}", UpdateTransactionHandler) // Update User's transaction.

	server.HandleFunc("DELETE /transactions/{id}", DeleteTransactionHandler) // Delete User's transaction.

	port := "8080"
	err = http.ListenAndServe(":"+port, server)
	if err != nil {
		fmt.Println("failed to start server: ", err)
		return
	}
	fmt.Println("Server is running on port: ", port)
}
