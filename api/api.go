package api

import (
	"encoding/json"
	"fmt"

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

	var newTransactionReq CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&newTransactionReq); err != nil {
		msg := fmt.Sprintf("failed to parse save transaction request: %v", err)
		return iz.Respond().Status(500).Text(msg)
	}

	newTransaction := budget.TransactionRequest{
		CategoryName: newTransactionReq.CategoryName,
		CategoryType: newTransactionReq.CategoryType,
		Amount:       newTransactionReq.Amount,
		Currency:     newTransactionReq.Currency,
		Note:         newTransactionReq.Note,
	}

	if err := api.Service.SaveTransaction(userId, newTransaction); err != nil {
		msg := fmt.Sprintf("failed to create transaction: %v", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}
	msg := fmt.Sprintf("transaction successfully created")
	return iz.Respond().Status(201).Text(msg)
}

func (api *Api) SaveExpenseCategoryHandler(r *iz.Request) iz.Responder {
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

	var newExpCategoryReq ExpenseCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&newExpCategoryReq); err != nil {
		msg := fmt.Sprintf("failed to parse expense category request: %v", err)
		return iz.Respond().Status(500).Text(msg)
	}

	if newExpCategoryReq.Name == "" {
		msg := fmt.Sprintf("category name is required")
		return iz.Respond().Status(400).Text(msg)
	}
	if newExpCategoryReq.MaxAmount <= 0 {
		msg := fmt.Sprintf("category max amount should be greater than 0")
		return iz.Respond().Status(400).Text(msg)
	}
	if newExpCategoryReq.PeriodDay < 0 {
		msg := fmt.Sprintf("category period day should be positive")
		return iz.Respond().Status(400).Text(msg)
	}

	newExpCategory := budget.ExpenseCategoryRequest{
		Name:      newExpCategoryReq.Name,
		MaxAmount: newExpCategoryReq.MaxAmount,
		PeriodDay: newExpCategoryReq.PeriodDay,
		Note:      newExpCategoryReq.Note,
		Type:      "-",
	}

	if err := api.Service.SaveExpenseCategory(userId, newExpCategory); err != nil {
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}

	msg := fmt.Sprintf("category successfully created")
	return iz.Respond().Status(201).Text(msg)
}

func (api *Api) SaveIncomeCategoryHandler(r *iz.Request) iz.Responder {
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

	var newIncCategoryReq IncomeCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&newIncCategoryReq); err != nil {
		msg := fmt.Sprintf("failed to parse income category request: %v", err)
		return iz.Respond().Status(500).Text(msg)
	}

	if newIncCategoryReq.Name == "" {
		msg := fmt.Sprintf("category name is required")
		return iz.Respond().Status(400).Text(msg)
	}

	newIncCategory := budget.IncomeCategoryRequest{
		Name:         newIncCategoryReq.Name,
		TargetAmount: newIncCategoryReq.TargetAmount,
		Note:         newIncCategoryReq.Note,
		Type:         "+",
	}

	if err := api.Service.SaveIncomeCategory(userId, newIncCategory); err != nil {
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}

	msg := fmt.Sprintf("category successfully created")
	return iz.Respond().Status(201).Text(msg)

}

func (api *Api) GetFilteredIncomeCategoriesHandler(r *iz.Request) iz.Responder {
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

	filter, err := IncomeCategoryCheckParams(params)
	if err != nil {
		msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	categories, err := api.Service.GetFilteredIncomeCategories(userId, filter)

	if err != nil {
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}
	var categoryList ListIncomeCategories
	categoryList.Categories = make([]IncomeCategoryResponseItem, 0, len(categories))
	for _, c := range categories {
		categoryList.Categories = append(categoryList.Categories, IncomeCategoryToHttp(c))
	}

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) GetFilteredExpenseCategoriesHandler(r *iz.Request) iz.Responder {
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

	filter, err := ExpenseCategoryCheckParams(params)
	if err != nil {
		msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	categories, err := api.Service.GetFilteredExpenseCategories(userId, filter)
	if err != nil {
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}
	var categoryList ListExpenseCategories
	categoryList.Categories = make([]ExpenseCategoryResponseItem, 0, len(categories))
	for _, c := range categories {
		categoryList.Categories = append(categoryList.Categories, ExpenseCategoryToHttp(c))
	}

	return iz.Respond().Status(200).JSON(categoryList)
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

	filter, err := TransactionCheckParams(params)
	if err != nil {
		msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	transactions, err := api.Service.GetFilteredTransactions(userId, filter)
	if err != nil {
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}

	var transactionList ListTransactionResponse
	transactionList.Transactions = make([]TransactionItem, 0, len(transactions))

	for _, transaction := range transactions {
		transactionList.Transactions = append(transactionList.Transactions, TransactionToHttp(transaction))
	}

	return iz.Respond().Status(200).JSON(transactionList)
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

	txnId := r.PathValue("id")

	txn, err := api.Service.GetTranscationById(userId, txnId)
	if err != nil {
		logging.Logger.Errorf("failed to get transaction by id request: %v", err)
		msg := fmt.Sprintf("failed to get transaction by id: %s", err.Error())
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	var transactionList ListTransactionResponse
	transactionList.Transactions = make([]TransactionItem, 0, 1)
	transactionList.Transactions = append(transactionList.Transactions, TransactionToHttp(txn))

	return iz.Respond().Status(200).JSON(transactionList)
}

func (api *Api) UpdateExpenseCategoryHandler(r *iz.Request) iz.Responder {
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

	var updateExpCategoryReq UpdateExpenseCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&updateExpCategoryReq); err != nil {
		msg := fmt.Sprintf("failed to parse expense category update request: %v", err)
		return iz.Respond().Status(500).Text(msg)
	}

	if updateExpCategoryReq.NewName == "" {
		msg := fmt.Sprintf("category name is required")
		return iz.Respond().Status(400).Text(msg)
	}

	updateExpCategoryItem := budget.UpdateExpenseCategoryRequest{
		ID:           updateExpCategoryReq.ID,
		NewName:      updateExpCategoryReq.NewName,
		NewMaxAmount: updateExpCategoryReq.NewMaxAmount,
		NewPeriodDay: updateExpCategoryReq.NewPeriodDay,
		NewNote:      updateExpCategoryReq.NewNote,
	}

	updatedCategory, err := api.Service.UpdateExpenseCategory(userId, updateExpCategoryItem)
	if err != nil {
		msg := fmt.Sprintf("failed to update expense category: %v", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	var categoryList ListExpenseCategories
	categoryList.Categories = make([]ExpenseCategoryResponseItem, 0, 1)
	categoryList.Categories = append(categoryList.Categories, ExpenseCategoryToHttp(*updatedCategory))

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) DeleteExpenseCategoryHandler(r *iz.Request) iz.Responder {
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

	var txnId string = r.PathValue("id")
	if txnId == "" {
		msg := fmt.Sprintf("category ID is required")
		return iz.Respond().Status(400).Text(msg)
	}

	if err := api.Service.DeleteExpenseCategory(userId, txnId); err != nil {
		msg := fmt.Sprintf("failed to delete expense category: %v", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	msg := fmt.Sprintf("category successfully deleted")
	return iz.Respond().Status(200).Text(msg)
}

func (api *Api) UpdateIncomeCategoryHandler(r *iz.Request) iz.Responder {
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

	var updateIncCategoryReq UpdateIncomeCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&updateIncCategoryReq); err != nil {
		msg := fmt.Sprintf("failed to parse income category update request: %v", err)
		return iz.Respond().Status(500).Text(msg)
	}

	if updateIncCategoryReq.NewName == "" {
		msg := fmt.Sprintf("category name is required")
		return iz.Respond().Status(400).Text(msg)
	}

	updateIncCategoryItem := budget.UpdateIncomeCategoryRequest{
		ID:              updateIncCategoryReq.ID,
		NewName:         updateIncCategoryReq.NewName,
		NewTargetAmount: updateIncCategoryReq.NewTargetAmount,
		NewNote:         updateIncCategoryReq.NewNote,
	}

	updatedCategory, err := api.Service.UpdateIncomeCategory(userId, updateIncCategoryItem)
	if err != nil {
		msg := fmt.Sprintf("failed to update income category: %v", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	var categoryList ListIncomeCategories
	categoryList.Categories = make([]IncomeCategoryResponseItem, 0, 1)
	categoryList.Categories = append(categoryList.Categories, IncomeCategoryToHttp(*updatedCategory))

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) DeleteIncomeCategoryHandler(r *iz.Request) iz.Responder {
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

	var tId string = r.PathValue("id")
	if tId == "" {
		msg := fmt.Sprintf("category ID is required")
		return iz.Respond().Status(400).Text(msg)
	}
	if err := api.Service.DeleteIncomeCategory(userId, tId); err != nil {
		msg := fmt.Sprintf("failed to delete income category: %v", err)
		return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	}

	msg := fmt.Sprintf("category successfully deleted")
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
		return iz.Respond().Status(httpStatusFromError(err)).Text(err.Error())
	}
	msg := "Logout successful."
	return iz.Respond().Status(200).Text(msg)
}
