package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/0xcafe-io/iz"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/logging"
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
	var newUserReq SaveUserRequest
	if err := json.NewDecoder(r.Body).Decode(&newUserReq); err != nil {
		msg := fmt.Sprintf("invalid request body: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	newUser := auth.NewUser{
		UserName:      newUserReq.UserName,
		FullName:      newUserReq.FullName,
		PasswordPlain: newUserReq.Password,
		Email:         newUserReq.Email,
	}

	if err := newUser.ValidateUserFields(); err != nil {
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}

	token, err := api.Service.SaveUser(newUser)
	if err != nil {
		msg := fmt.Sprintf("registration failed: %v", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	resp := UserCreatedResponse{
		Message: "Registration Completed",
		Token:   token,
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
		logging.Logger.Errorf("Failed to parse save transaction request: %v", err)
		msg := fmt.Sprintf("failed to parse save transaction request")
		return iz.Respond().Status(500).Text(msg)
	}
	defer r.Body.Close()

	if err := api.Service.SaveTransaction(userId, newTransaction.Amount, newTransaction.Limit, newTransaction.Category, newTransaction.Ttype, newTransaction.Currency); err != nil {
		logging.Logger.Errorf("Failed to create transaction request: %v", err)
		msg := fmt.Sprintf("failed to create transaction")
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	msg := fmt.Sprintf("transaction successfully created")
	return iz.Respond().Status(201).Text(msg)
}

func (api *Api) SaveCategoryHandler(r *iz.Request) iz.Responder {
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

	var newCategory NewCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&newCategory); err != nil {
		logging.Logger.Errorf("Failed to parse save category request: %v", err)
		msg := fmt.Sprintf("failed to parse save category request:")
		return iz.Respond().Status(500).Text(msg)
	}
	defer r.Body.Close()

	if err := api.Service.SaveCategory(userId, newCategory.Name, newCategory.Type, newCategory.MaxAmount, newCategory.PeriodDays); err != nil {
		msg := fmt.Sprintf("failed to save category: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	msg := fmt.Sprintf("category saved successfully.")
	return iz.Respond().Status(200).Text(msg)
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

	filters, err := ListValidateParams(params)
	if err != nil {
		msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	ts, err := api.Service.GetFilteredTransactions(userId, filters)
	if err != nil {
		logging.Logger.Errorf("Failed to get filtered transactions request: %v", err)
		msg := fmt.Sprintf("failed to get transactions")
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	var tsContainer ListTransactionResponse

	tsForHttp := make([]TransactionItem, 0, len(ts))

	for _, t := range ts {
		tForHttp := TransactionToHttp(t)
		tsContainer.Transactions = append(tsContainer.Transactions, tForHttp)
	}
	return iz.Respond().Status(200).JSON(tsForHttp)
}

func (api *Api) GetFilteredCategoriesHandler(r *iz.Request) iz.Responder {
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

	filters, err := CategoriesListValidateParams(params)
	if err != nil {
		msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	categories, err := api.Service.GetFilteredCategories(userId, filters)
	if err != nil {
		logging.Logger.Errorf("Failed to get filtered categories: %v", err)
		msg := fmt.Sprintf("failed to get filtered categories: %v", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	categoriesForHttp := make([]CategoryItem, 0, len(categories))

	for _, category := range categories {
		categoryForHttp := CategoryToHttp(category)
		categoriesForHttp = append(categoriesForHttp, categoryForHttp)
	}
	fmt.Println(filters)
	return iz.Respond().Status(200).JSON(categoriesForHttp)
}

func (api *Api) GetTotals(r *iz.Request) iz.Responder {
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
		msg := fmt.Sprintf("invalid parameters: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	result, err := api.Service.GetTotals(userId, *filterFields)
	if err != nil {
		logging.Logger.Errorf("Failed to get total by type and currency request: %v", err)
		msg := fmt.Sprintf("failed to get totals by type and currency")
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	resultForHttp := GetTotalsResponse{
		Currency: result.Currency,
		Type:     result.Type,
		Total:    result.Total,
	}
	return iz.Respond().Status(200).JSON(resultForHttp)
}

func (api *Api) GetCategoryByNameHandler(r *iz.Request) iz.Responder {
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

	_ = userId

	// cName := r.PathValue("name")

	// t, err := api.Service.GetCategoryByName(userId, tId)
	if err != nil {
		logging.Logger.Errorf("Failed to get transaction by Id request: %v", err)
		msg := fmt.Sprintf("failed to get transaction by ID: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	// tForHttp := TransactionToHttp()
	return iz.Respond().Status(403).JSON("failed")
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

	tId := r.PathValue("id")

	t, err := api.Service.GetTranscationById(userId, tId)
	if err != nil {
		logging.Logger.Errorf("Failed to get transaction by Id request: %v", err)
		msg := fmt.Sprintf("failed to get transaction by ID: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	tForHttp := TransactionToHttp(t)
	return iz.Respond().Status(200).JSON(tForHttp)
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

	tId := r.PathValue("id")
	params := r.URL.Query()

	tType := params.Get("type")
	amountString := params.Get("amount")
	if tType != "" && amountString != "" {
		amountFloat, err := strconv.ParseFloat(amountString, 64)
		if err != nil {
			logging.Logger.Errorf("Failed to parse change transaction amount: %v", err)
			msg := fmt.Sprintf("failed to parse amount")
			return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
		}

		err = api.Service.ChangeAmountOfTransaction(userId, tId, tType, amountFloat)
		if err != nil {
			logging.Logger.Errorf("Failed to change transaction amount: %v", err)
			msg := fmt.Sprintf("failed to change transaction amount")
			return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
		}
		msg := fmt.Sprintf("amount successfully changed")
		return iz.Respond().Status(200).Text(msg)
	}

	var updateReq UpdateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		msg := fmt.Sprintf("Invalid request payload %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	updatedReq := budget.UpdateTransactionItem{
		ID:          tId,
		Amount:      updateReq.Amount,
		Limit:       updateReq.Limit,
		Currency:    updateReq.Currency,
		Category:    updateReq.Category,
		UpdatedDate: time.Now(),
		Type:        updateReq.Type,
	}

	defer r.Body.Close()

	if err := api.Service.UpdateTransaction(userId, updatedReq); err != nil {
		logging.Logger.Errorf("Failed to update transaction request: %v", err)
		msg := fmt.Sprintf("failed to update transaction")
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
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

	tId := r.PathValue("id")

	if err := api.Service.DeleteTransaction(userId, tId); err != nil {
		logging.Logger.Errorf("Failed to delete transaction request: %v", err)
		msg := fmt.Sprintf("failed to delete transaction")
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	msg := fmt.Sprintf("transaction deleted successfully")
	return iz.Respond().Status(200).Text(msg)
}

func (api *Api) LoginUserHandler(r *iz.Request) iz.Responder {
	var loginRequest UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginRequest); err != nil {
		msg := fmt.Sprintf("invalid request body")
		return iz.Respond().Status(400).Text(msg)
	}

	credentials := auth.UserCredentialsPure{
		UserName:      loginRequest.UserName,
		PasswordPlain: loginRequest.Password,
	}

	response := LoginResponse{}

	token, err := api.Service.GenerateSession(credentials)
	if err != nil {
		response.Message = err.Error()
		return iz.Respond().Status(httpStatusFromError(err)).JSON(response)
	}
	response.Message = "You've logged in successfully!"
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
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	if err := api.Service.LogoutUser(userId, token); err != nil {
		msg := fmt.Sprintf("logout failed: %w", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	msg := "Logout successful."
	return iz.Respond().Status(200).Text(msg)
}
