package api

import (
	"encoding/json"
	"fmt"
	"strconv"

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
		logging.Logger.Errorf("Failed to parse save transaction request: %v", err)
		msg := fmt.Sprintf("failed to parse save transaction request: %v", err)
		return iz.Respond().Status(500).Text(msg)
	}
	amountStr := newTransactionReq.Amount

	if len(amountStr) == 0 || (amountStr[0] != '+' && amountStr[0] != '-') {
		msg := fmt.Sprintf("invalid transaction amount format: '%s'", amountStr)
		return iz.Respond().Status(400).Text(msg)
	}

	amountValue, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		msg := fmt.Sprintf("failed to convert amount: %v", err)
		return iz.Respond().Status(400).Text(msg)
	}

	newTransaction := budget.TransactionRequest{
		Category: newTransactionReq.Category,
		Amount:   amountValue,
		Currency: newTransactionReq.Currency,
		Note:     newTransactionReq.Note,
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
		msg := fmt.Sprintf("failed to parse save category request: %v", err)
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

	newExpCategory := budget.CategoryRequest{
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

func (api *Api) GetFilteredTransactionsHandler(r *iz.Request) iz.Responder {
	token := r.Header.Get("Authorization")
	if token == "" {
		msg := fmt.Sprintf("authorization failed: Authorization header is required.")
		return iz.Respond().Status(401).Text(msg)
	}

	_, err := api.Service.CheckSession(token)
	if err != nil {
		msg := fmt.Sprintf("authorization failed: %s", err.Error())
		return iz.Respond().Status(401).Text(msg)
	}

	params := r.URL.Query()

	_, err = ListValidateParams(params)
	if err != nil {
		msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
		return iz.Respond().Status(400).Text(msg)
	}

	// ts, err := api.Service.GetFilteredTransactions(userId, filters)
	// if err != nil {
	// 	logging.Logger.Errorf("Failed to get filtered transactions request: %v", err)
	// 	msg := fmt.Sprintf("failed to get transactions")
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }
	// var tsContainer ListTransactionResponse

	// tsForHttp := make([]TransactionItem, 0, len(ts))

	// for _, t := range ts {
	// 	tForHttp := TransactionToHttp(t)
	// 	tsContainer.Transactions = append(tsContainer.Transactions, tForHttp)
	// }
	return iz.Respond().Status(200).JSON("success")
}

func (api *Api) GetFilteredCategoriesHandler(r *iz.Request) iz.Responder {
	token := r.Header.Get("Authorization")
	if token == "" {
		msg := fmt.Sprintf("authorization failed: Authorization header is required.")
		return iz.Respond().Status(401).Text(msg)
	}

	// userId, err := api.Service.CheckSession(token)
	// if err != nil {
	msg := fmt.Sprintf("authorization failed: %s", "test")
	return iz.Respond().Status(401).Text(msg)
	//}

	// params := r.URL.Query()

	// filters, err := CategoriesListValidateParams(params)
	// if err != nil {
	// msg := fmt.Sprintf("invalid filter parameteres: %s", err.Error())
	// return iz.Respond().Status(400).Text(msg)
	// }

	// _, err = api.Service.(userId, filters)
	// if err != nil {
	// 	logging.Logger.Errorf("Failed to get filtered categories: %v", err)
	// 	msg := fmt.Sprintf("failed to get filtered categories: %v", err.Error())
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }

	// categoriesForHttp := make([]CategoryItem, 0, len(categories))

	// for _, category := range categories {
	// 	categoryForHttp := CategoryToHttp(category)
	// 	categoriesForHttp = append(categoriesForHttp, categoryForHttp)
	// }
	// fmt.Println(filters)

	// return iz.Respond().Status(200).JSON(categoriesForHttp
	// return iz.Respond().Status(200).JSON("success")
}

func (api *Api) GetTotals(r *iz.Request) iz.Responder {
	// token := r.Header.Get("Authorization")
	// if token == "" {
	// 	msg := fmt.Sprintf("authorization failed: Authorization header is required.")
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// userId, err := api.Service.CheckSession(token)
	// if err != nil {
	// 	msg := fmt.Sprintf("authorization failed: %s", err.Error())
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// params := r.URL.Query()

	// filters := &budget.GetTotals{}
	// filterFields, err := filters.GetTotalValidate(params)
	// if err != nil {
	// 	msg := fmt.Sprintf("invalid parameters: %s", err.Error())
	// 	return iz.Respond().Status(400).Text(msg)
	// }

	// result, err := api.Service.(userId, *filterFields)
	// if err != nil {
	// 	logging.Logger.Errorf("Failed to get total by type and currency request: %v", err)
	// 	msg := fmt.Sprintf("failed to get totals by type and currency")
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }

	// resultForHttp := GetTotalsResponse{
	// 	Currency: result.Currency,
	// 	Type:     result.Type,
	// 	Total:    result.Total,
	// }
	return iz.Respond().Status(200).JSON("")
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
	// token := r.Header.Get("Authorization")
	// if token == "" {
	// 	msg := fmt.Sprintf("authorization failed: Authorization header is required.")
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// userId, err := api.Service.CheckSession(token)
	// if err != nil {
	// 	msg := fmt.Sprintf("authorization failed: %s", err.Error())
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// tId := r.PathValue("id")

	// t, err := api.Service.GetTranscationById(userId, tId)
	// if err != nil {
	// 	logging.Logger.Errorf("Failed to get transaction by Id request: %v", err)
	// 	msg := fmt.Sprintf("failed to get transaction by ID: %s", err.Error())
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }
	// tForHttp := TransactionToHttp(t)
	return iz.Respond().Status(200).JSON("")
}

func (api *Api) DeleteTransactionHandler(r *iz.Request) iz.Responder {
	// token := r.Header.Get("Authorization")
	// if token == "" {
	// 	msg := fmt.Sprintf("authorization failed: Authorization header is required.")
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// userId, err := api.Service.CheckSession(token)
	// if err != nil {
	// 	msg := fmt.Sprintf("authorization failed: %s", err.Error())
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// tId := r.PathValue("id")

	// if err := api.Service.DeleteTransaction(userId, tId); err != nil {
	// 	logging.Logger.Errorf("Failed to delete transaction request: %v", err)
	// 	msg := fmt.Sprintf("failed to delete transaction")
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }
	// msg := fmt.Sprintf("transaction deleted successfully")
	return iz.Respond().Status(200).Text("")
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
	// token := r.Header.Get("Authorization")
	// if token == "" {
	// 	msg := fmt.Sprintf("authorization failed: Authorization header is required.")
	// 	return iz.Respond().Status(401).Text(msg)
	// }

	// userId, err := api.Service.CheckSession(token)
	// if err != nil {
	// 	msg := fmt.Sprintf("authorization failed: %s", err.Error())
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }

	// if err := api.Service.LogoutUser(userId, token); err != nil {
	// 	msg := fmt.Sprintf("logout failed: %w", err)
	// 	return iz.Respond().Status(httpStatusFromError(err)).Text(msg)
	// }
	msg := "Logout successful."
	return iz.Respond().Status(200).Text(msg)
}
