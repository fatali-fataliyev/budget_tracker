package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	params := r.URL.Query()

	filters := &budget.ListTransactionsFilters{}
	filterFields, err := filters.ListValidateParams(params)
	if err != nil {
		msg := fmt.Sprintf("something went wrong: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	ts, err := api.Service.GetFilteredTransactions(userId, *filterFields)
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

func (api *Api) GetTotalsByTypeAndCurrencyHandler(r *iz.Request) iz.Responder {
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

	params := r.URL.Query()

	filters := &budget.GetTotals{}
	filterFields, err := filters.GetTotalValidate(params)
	if err != nil {
		msg := fmt.Sprintf("something went wrong: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	result, err := api.Service.GetTotalsByTypeAndCurrency(userId, *filterFields)
	if err != nil {
		msg := fmt.Sprintf("failed to get totals by type and currency: %s", err.Error())
		return iz.Respond().Status(401).Text(msg)
	}

	resultForHttp := GetTotalsResponse{
		Currency: result.Currency,
		Type:     result.Type,
		Total:    result.Total,
	}
	return iz.Respond().Status(200).JSON(resultForHttp)
}

func (api *Api) GetTransactionByIdHandler(r *iz.Request) iz.Responder {
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

	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] == "" {
		msg := fmt.Sprintf("Missing transaction ID:")
		return iz.Respond().Status(401).Text(msg)
	}
	tId := parts[2]

	t, err := api.Service.GetTranscationById(userId, tId)
	if err != nil {
		msg := fmt.Sprintf("failed to get transaction by ID: %s", err.Error())
		return iz.Respond().Status(500).Text(msg)
	}
	tForHttp := TransactionToHttp(t)
	return iz.Respond().Status(200).JSON(tForHttp)
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

func (api *Api) UpdateTransactionHandler(r *iz.Request) iz.Responder {
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

	var updateReq UpdateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		msg := fmt.Sprintf("Invalid request payload %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		msg := fmt.Sprintf("Missing transaction ID")
		return iz.Respond().Status(401).Text(msg)
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

	if err := api.Service.UpdateTransaction(userId, updatedReq); err != nil {
		msg := fmt.Sprintf("failed to update transaction: %v", err)
		return iz.Respond().Status(400).Text(msg)
	}

	msg := fmt.Sprintf("transaction updated successfully")
	return iz.Respond().Status(200).Text(msg)
}

func (api *Api) DeleteTransactionHandler(r *iz.Request) iz.Responder {
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

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		msg := fmt.Sprintf("Missing transaction ID")
		return iz.Respond().Status(401).Text(msg)
	}
	tId := parts[2]

	if err := api.Service.DeleteTransaction(userId, tId); err != nil {
		msg := fmt.Sprintf("failed to delete transaction: %s", err.Error())
		return iz.Respond().Status(401).Text(msg)
	}
	msg := fmt.Sprintf("transaction deleted successfully")
	return iz.Respond().Status(200).Text(msg)
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
