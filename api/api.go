package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/0xcafe-io/iz"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
)

type Api struct {
	Service *budget.BudgetTracker
}

func NewApi(service *budget.BudgetTracker) *Api {
	return &Api{
		Service: service,
	}
}

func (api *Api) SaveUserHandler(r *iz.Request) iz.Responder {
	var newUser auth.NewUser
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		msg := fmt.Sprintf("invalid request body: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	err := newUser.Validate()
	if err != nil {
		msg := fmt.Sprintf("invalid request body: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	defer r.Body.Close()
	if err := api.Service.SaveUser(newUser); err != nil {
		msg := fmt.Sprintf("invalid request body: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	resp := UserCreatedResponse{
		Message: "registration completed, please login again",
	}
	return iz.Respond().Status(201).JSON(resp)
}

func (api *Api) SaveTransactionHandler(r *iz.Request) iz.Responder {
	token := r.Header.Get("Authorization")
	if token == "" {
		msg := fmt.Sprintf("authorization failed: Authorization header is required.")
		return iz.Respond().Status(401).Text(msg)
	}

	userId, err := api.Service.CheckSession(token)
	if err != nil {
		msg := fmt.Sprintf("authorization failed: %s", err.Error())
		return iz.Respond().Status(401).Text(msg)
	}

	var newTransaction CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&newTransaction); err != nil {
		msg := fmt.Sprintf("invalid request body: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	defer r.Body.Close()

	if err := api.Service.SaveTransaction(userId, newTransaction.Amount, newTransaction.Category, newTransaction.Ttype, newTransaction.Currency); err != nil {
		msg := fmt.Sprintf("create transaction failed: %s", err.Error())
		return iz.Respond().Status(500).Text(msg)
	}

	msg := fmt.Sprintf("transaction successfully created")
	return iz.Respond().Status(201).Text(msg)

}

func (api *Api) GetFilteredTransactionsHandler(r *iz.Request) iz.Responder {
	token := r.Header.Get("Authorization")
	if token == "" {
		msg := fmt.Sprintf("authorization failed: Authorization header is required.")
		return iz.Respond().Status(401).Text(msg)
	}

	userId, err := api.Service.CheckSession(token)
	if err != nil {
		msg := fmt.Sprintf("authorization failed: %s", err.Error())
		return iz.Respond().Status(401).Text(msg)
	}

	// TODO extract a function to parse ListTransactionsFilters
	params := r.URL.Query()

	filterFields := budget.ListTransactionsFilters{}

	if len(params) == 0 {
		filterFields.IsAllNil = true
		ts, err := api.Service.GetFilteredTransactions(userId, filterFields)
		if err != nil {
			msg := fmt.Sprintf("failed to get transactions: %s", err.Error())
			return iz.Respond().Status(400).Text(msg)
		}

		tsForHttp := make([]TransactionItem, 0, len(ts))

		for _, t := range ts {
			tForHttpItem := TransactionToHttp(t)
			tsForHttp = append(tsForHttp, tForHttpItem)
		}

		return iz.Respond().Status(200).JSON(tsForHttp)
	}

	var categoriesWithEmptyStrings []string
	var categories []string
	categoryRaw := params.Get("categories")

	categoriesWithEmptyStrings = strings.Split(categoryRaw, ",")

	for _, category := range categoriesWithEmptyStrings {
		trimmed := strings.TrimSpace(category)
		if trimmed != "" {
			categories = append(categories, trimmed)
		}
	}

	transactionType := params.Get("type")
	minAmount := params.Get("min")
	maxAmount := params.Get("max")

	var maxAmountFloat *float64
	var minAmountFloat *float64

	if minAmount != "" {
		parsedMinAmount, err := strconv.ParseFloat(minAmount, 64)
		if err == nil {
			minAmountFloat = &parsedMinAmount
		}
		if err != nil {
			msg := fmt.Sprintf("invalid minimum amount %s", err.Error())
			return iz.Respond().Status(400).Text(msg)
		}
	} else {
		minAmountFloat = nil
	}
	if maxAmount != "" {
		parsedMaxAmount, err := strconv.ParseFloat(maxAmount, 64)
		if err == nil {
			maxAmountFloat = &parsedMaxAmount
		}
		if err != nil {
			msg := fmt.Sprintf("invalid maximum amount %s", err.Error())
			return iz.Respond().Status(400).Text(msg)
		}
	} else {
		minAmountFloat = nil
	}
	if transactionType != "" {
		if transactionType == "income" {
			income := "+"
			filterFields.Type = &income
			fmt.Println(*filterFields.Type)
		} else if transactionType == "expense" {
			expense := "-"
			filterFields.Type = &expense
			fmt.Println(*filterFields.Type)
		} else {
			msg := fmt.Sprintf("invalid transaction type")
			return iz.Respond().Status(400).Text(msg)
		}
	}

	filterFields.MinAmount = minAmountFloat
	filterFields.MaxAmount = maxAmountFloat
	filterFields.Categories = categories

	ts, err := api.Service.GetFilteredTransactions(userId, filterFields)
	if err != nil {
		msg := fmt.Sprintf("failed to get transactions: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	tsForHttp := make([]TransactionItem, 0, len(ts))

	for _, t := range ts {
		tForHttpItem := TransactionToHttp(t)
		tsForHttp = append(tsForHttp, tForHttpItem)
	}
	return iz.Respond().Status(200).JSON(tsForHttp)
}

func (api *Api) GetTotalsByTypeHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

}

func (api *Api) GetTransactionByIdHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "Missing transaction ID", http.StatusBadRequest)
		return
	}
	id := parts[2]

	t, err := api.Service.GetTranscationById(token, id)
	if err != nil {
		http.Error(w, "Transaction not found"+err.Error(), http.StatusNotFound)
		return
	}

	tForHttp := TransactionToHttp(t)

	data, err := json.Marshal(tForHttp)
	if err != nil {
		http.Error(w, "Failed to marshal transaction", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (api *Api) ValidateUser(w http.ResponseWriter, r *http.Request) {
	var loginReq UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	if loginReq.UserName == "" || loginReq.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	credentials := auth.UserCredentialsPure{
		UserName:      loginReq.UserName,
		PasswordPlain: loginReq.Password,
	}

	_, err := api.Service.ValidateUser(credentials)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to validate user: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("login successfully"))
}

func (api *Api) UpdateTransactionHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

	var updateReq UpdateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "Missing transaction ID", http.StatusBadRequest)
		return
	}
	tId := parts[2]

	updatedReq := budget.UpdateTransactionItem{
		ID:          tId,
		Amount:      updateReq.Amount,
		Currency:    updateReq.Currency,
		Category:    updateReq.Category,
		UpdatedDate: time.Now(),
		Type:        updateReq.Type,
	}
	defer r.Body.Close()

	if err := api.Service.UpdateTransaction(token, updatedReq); err != nil {
		http.Error(w, fmt.Sprintf("failed to update transaction: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("[+]transaction updated"))
}

func (api *Api) DeleteTransactionHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "Missing transaction ID", http.StatusBadRequest)
		return
	}
	tId := parts[2]

	if err := api.Service.DeleteTransaction(token, tId); err != nil {
		http.Error(w, fmt.Sprintf("failed to delete transaction: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("[+]transaction deleted"))
}

func (api *Api) LoginUserHandler(r *iz.Request) iz.Responder {
	var loginRequest UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginRequest); err != nil {
		msg := fmt.Sprintf("login failed: internal error")
		return iz.Respond().Status(500).Text(msg)
	}
	fmt.Println(loginRequest.UserName)
	fmt.Println(loginRequest.Password)
	credentials := auth.UserCredentialsPure{
		UserName:      loginRequest.UserName,
		PasswordPlain: loginRequest.Password,
	}

	response := AuthenticationResponse{}

	token, err := api.Service.GenerateSession(credentials)
	if err != nil {
		response.Message = err.Error()
		return iz.Respond().Status(500).JSON(response)
	}
	response.Message = "login successful"
	response.Token = token
	return iz.Respond().Status(200).JSON(response)
}

func (api *Api) LogoutUserHandler(r *iz.Request) iz.Responder {
	token := r.Header.Get("Authorization")
	if token == "" {
		msg := fmt.Sprintf("authorization failed: Authorization header is required.")
		return iz.Respond().Status(401).Text(msg)
	}

	userId, err := api.Service.CheckSession(token)
	if err != nil {
		msg := fmt.Sprintf("authorization failed: %s", err.Error())
		return iz.Respond().Status(401).Text(msg)
	}

	if err := api.Service.LogoutUser(userId, token); err != nil {
		msg := fmt.Sprintf("logout failed: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}
	msg := fmt.Sprintf("logout successful")
	return iz.Respond().Status(200).Text(msg)
}

func TransactionToHttp(transcation budget.Transaction) TransactionItem {
	return TransactionItem{
		ID:          transcation.ID,
		Amount:      transcation.Amount,
		Currency:    transcation.Currency,
		Category:    transcation.Category,
		CreatedDate: transcation.CreatedDate.Format("02/01/2006 15:04"),
		UpdatedDate: transcation.UpdatedDate.Format("02/01/2006 15:04"),
		Type:        transcation.Type,
		CreatedBy:   transcation.CreatedBy,
	}
}
