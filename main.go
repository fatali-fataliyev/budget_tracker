package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	inMem "github.com/fatali-fataliyev/budget_tracker/internal/storage"
)

var bt budget.BudgetTracker // Global

func main() {

	settingsForInmemory := auth.UserCredentialsPure{
		UserName:      "john_doe",
		PasswordPlain: "johnBudget",
	}

	bt = budget.NewBudgetTracker(inMem.NewInternalMemeoryStorage(settingsForInmemory))

	reader := bufio.NewReader(os.Stdin)
	if bt.StorageType == "inmemory" {
		for {
			fmt.Println("\n--- Budget CLI ---")
			fmt.Println("1. Save user")
			fmt.Println("2. Save transaction")
			fmt.Println("3. Get all transactions")
			fmt.Println("4. Get transactions by type")
			fmt.Println("5. Get transaction by ID")
			fmt.Println("6. Delete transaction")
			fmt.Println("7. Generate session")
			fmt.Println("0. Exit")
			fmt.Print("Select an option: ")

			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)

			switch choice {
			case "1":
				saveUser(reader)
			case "2":
				saveTransaction(reader)
			case "3":
				getAllTransactions(reader)
			case "4":
				getTransactionsByType(reader)
			case "5":
				getTransactionByID(reader)
			case "6":
				deleteTransaction(reader)
			case "7":
				generateSession(reader)
			case "0":
				fmt.Println("Exiting...")
				return
			default:
				fmt.Println("Invalid option.")
			}
		}
	} else {
		fmt.Println("external memory connecting. . .")
	}

}

func saveUser(reader *bufio.Reader) {
	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	fmt.Print("Fullname: ")
	fullname, _ := reader.ReadString('\n')
	fmt.Print("Nickname(optional): ")
	nickname, _ := reader.ReadString('\n')
	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')

	newUser := auth.NewUser{
		UserName:      strings.TrimSpace(username),
		FullName:      strings.TrimSpace(fullname),
		NickName:      strings.TrimSpace(nickname),
		Email:         strings.TrimSpace(email),
		PasswordPlain: strings.TrimSpace(password),
		PendingEmail:  strings.TrimSpace(email),
	}

	err := bt.SaveUser(newUser)
	if err != nil {
		fmt.Println("Error saving user:", err)
	} else {
		fmt.Println("User saved.")
	}
}

func saveTransaction(reader *bufio.Reader) {
	fmt.Print("Enter token: ")
	token, _ := reader.ReadString('\n')
	fmt.Print("Amount: ")
	amount, _ := reader.ReadString('\n')
	fmt.Print("Category: ")
	category, _ := reader.ReadString('\n')
	fmt.Print("Type (+ or -): ")
	tType, _ := reader.ReadString('\n')
	fmt.Print("Currency: ")
	currency, _ := reader.ReadString('\n')
	parsedAmount, err := strconv.ParseFloat(strings.TrimSpace(amount), 64)
	if err != nil {
		fmt.Println("Invalid amount. Please enter a valid number.")
		return
	}
	err = bt.SaveTransaction(token, parsedAmount, category, tType, currency)
	if err != nil {
		fmt.Println("Error saving transaction:", err)
	} else {
		fmt.Println("Transaction saved.")
	}
}

func getAllTransactions(reader *bufio.Reader) {
	fmt.Print("Enter token: ")
	token, _ := reader.ReadString('\n')
	ts, err := bt.GetAllTransactions(token)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, t := range ts {
		fmt.Println(t)
	}
}

func getTransactionsByType(reader *bufio.Reader) {
	fmt.Print("Enter token: ")
	token, _ := reader.ReadString('\n')
	fmt.Println("enter transaction type: ")
	tType, _ := reader.ReadString('\n')
	ts, err := bt.GetTransactionsByType(token, tType)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, t := range ts {
		fmt.Println(t)
	}
}

func getTransactionByID(reader *bufio.Reader) {
	fmt.Print("Enter token: ")
	token, _ := reader.ReadString('\n')
	fmt.Print("Transaction ID: ")
	tID, _ := reader.ReadString('\n')
	t, err := bt.GetTranscationById(token, strings.TrimSpace(tID))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(t)
}

func deleteTransaction(reader *bufio.Reader) {
	fmt.Print("Enter token: ")
	token, _ := reader.ReadString('\n')
	fmt.Print("Transaction ID: ")
	tID, _ := reader.ReadString('\n')

	err := bt.DeleteTransaction(token, strings.TrimSpace(tID))
	if err != nil {
		fmt.Println("Error deleting transaction:", err)
	} else {
		fmt.Println("Transaction deleted.")
	}
}

func generateSession(reader *bufio.Reader) {
	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	credentialsPure := auth.UserCredentialsPure{
		UserName:      strings.TrimSpace(username),
		PasswordPlain: strings.TrimSpace(password),
	}
	session, err := bt.GenerateSession(credentialsPure)
	if err != nil {
		fmt.Println("Error generating session:", err)
	} else {
		fmt.Println("Session generated: token:", session)
	}
}
