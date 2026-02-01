package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/0xcafe-io/iz"
	appErrors "github.com/fatali-fataliyev/budget_tracker/customErrors"
	"github.com/fatali-fataliyev/budget_tracker/internal/auth"
	"github.com/fatali-fataliyev/budget_tracker/internal/budget"
	"github.com/fatali-fataliyev/budget_tracker/internal/contextutil"
	"github.com/fatali-fataliyev/budget_tracker/logging"
	"github.com/google/uuid"
	ocr "github.com/ranghetto/go_ocr_space"
)

const MAX_IMAGE_UPLOAD_SIZE = 1 << 20 // 1mib
const SUCCESS_CODE = "SUCCESS"
const FAIL_CODE = "FAIL"

type Api struct {
	Service *budget.BudgetTracker
}

func NewApi(service *budget.BudgetTracker) *Api {
	return &Api{
		Service: service,
	}
}

type contextKey string

const userIdKey contextKey = "userId"

func (api *Api) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			iz.Respond().Status(401).JSON(appErrors.ErrorResponse{
				Code:    appErrors.ErrAuth,
				Message: "Authorization header is required.",
			})
			return
		}
		ctx := r.Context()
		userId, err := api.Service.CheckSession(ctx, token)
		if err != nil {
			RespondError(err)
			return
		}

		newCtx := context.WithValue(ctx, userIdKey, userId)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}

func (api *Api) SaveUserHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	var newUserReq SaveUserRequest
	if err := json.NewDecoder(r.Body).Decode(&newUserReq); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Invalid request body",
		})
	}

	newUser := auth.NewUser{
		UserName:      newUserReq.UserName,
		FullName:      newUserReq.FullName,
		PasswordPlain: newUserReq.Password,
		Email:         newUserReq.Email,
	}

	if err := newUser.ValidateUserFields(); err != nil {
		return RespondError(err)
	}

	token, err := api.Service.SaveUser(ctx, newUser)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to save user | Error: %v", traceID, err)
		return RespondError(err)
	}

	return iz.Respond().Status(201).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Registration completed successfully",
		Extra:   token,
	})
}

func (api *Api) SaveTransactionHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var newTransactionReq CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&newTransactionReq); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Invalid request body",
		})
	}

	newTransaction := budget.TransactionRequest{
		CategoryId:   newTransactionReq.CategoryId,
		CategoryType: newTransactionReq.CategoryType,
		Amount:       newTransactionReq.Amount,
		Currency:     newTransactionReq.Currency,
		Note:         newTransactionReq.Note,
	}

	if err := api.Service.SaveTransaction(ctx, userId, newTransaction); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to save transaction | Error: %v", traceID, err)
		return RespondError(err)
	}

	return iz.Respond().Status(201).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Transaction posted.",
	})
}

func (api *Api) ProcessImageHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	_, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	err := r.ParseMultipartForm(MAX_IMAGE_UPLOAD_SIZE)
	if err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Maximum image size is 1MB",
		})
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Invalid Image",
		})
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, file); err != nil {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to process uploaded image",
		})
	}

	imageType := http.DetectContentType(buf.Bytes())
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	base64Img := fmt.Sprintf("data:%s;base64,%s", imageType, encoded)

	imageRawText, err := RequestOCRApi(ctx, base64Img)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to process reciept image | Error: %v", traceID, err)
		return RespondError(err)
	}

	processResultRaw, err := api.Service.ProcessImage(ctx, imageRawText)
	if err != nil {
		return RespondError(err)
	}

	processedImageResp := ProcessedImageToHttp(processResultRaw)
	return iz.Respond().Status(200).JSON(processedImageResp)
}

func RequestOCRApi(ctx context.Context, base64Img string) (string, error) {
	traceID := contextutil.TraceIDFromContext(ctx)

	ocrApiKey := os.Getenv("OCR_APIKEY")
	if ocrApiKey == "" {
		return "", fmt.Errorf("OCR key is required for processing transaction images")
	}

	config := ocr.InitConfig(ocrApiKey, "auto", ocr.OCREngine2)
	result, err := config.ParseFromBase64(base64Img)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to request Api.RequestOCRApi() | Error: %v", traceID, err)
		return "", appErrors.ErrorResponse{
			Code:    appErrors.ErrInternal,
			Message: "Failed to process image",
		}
	}

	return result.JustText(), nil
}

func (api *Api) SaveExpenseCategoryHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var newExpCategoryReq ExpenseCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&newExpCategoryReq); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Invalid request body",
		})
	}

	if newExpCategoryReq.Name == "" {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category name cannot be empty!",
		})
	}
	if newExpCategoryReq.MaxAmount <= 0 {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category maximum amount should be greater than 0",
		})
	}
	if newExpCategoryReq.PeriodDay < 0 {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category expire in day should be positive",
		})
	}

	newExpCategory := budget.ExpenseCategoryRequest{
		Name:      newExpCategoryReq.Name,
		MaxAmount: newExpCategoryReq.MaxAmount,
		PeriodDay: newExpCategoryReq.PeriodDay,
		Note:      newExpCategoryReq.Note,
		Type:      "-",
	}

	if err := api.Service.SaveExpenseCategory(ctx, userId, newExpCategory); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to save expense category | Error: %v", traceID, err)
		return RespondError(err)
	}

	return iz.Respond().Status(201).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Category created successfully.",
	})
}

func (api *Api) SaveIncomeCategoryHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var newIncCategoryReq IncomeCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&newIncCategoryReq); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Invalid request body",
		})
	}

	if newIncCategoryReq.Name == "" {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category name cannot be empty!",
		})
	}

	newIncCategory := budget.IncomeCategoryRequest{
		Name:         newIncCategoryReq.Name,
		TargetAmount: newIncCategoryReq.TargetAmount,
		Note:         newIncCategoryReq.Note,
		Type:         "+",
	}

	if err := api.Service.SaveIncomeCategory(ctx, userId, newIncCategory); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to save income category | Error: %v", traceID, err)
		return RespondError(err)
	}

	return iz.Respond().Status(201).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Category created successfully",
	})
}

func (api *Api) GetExpenseCategoryStatsHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	statsRaw, err := api.Service.GetExpenseCategoryStats(ctx, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get expense category stats | Error: %v", traceID, err)
		return RespondError(err)
	}
	stats := ExpenseStatsToHttp(statsRaw)

	return iz.Respond().Status(200).JSON(stats)
}

func (api *Api) GetIncomeCategoryStatsHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	statsRaw, err := api.Service.GetIncomeCategoryStats(ctx, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get income category stats | Error: %v", traceID, err)
		return RespondError(err)
	}
	stats := IncomeStatsToHttp(statsRaw)

	return iz.Respond().Status(200).JSON(stats)
}

func (api *Api) GetTransactionStatsHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	statsRaw, err := api.Service.GetTransactionStats(ctx, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get transactions stats | Error: %v", traceID, err)
		return RespondError(err)
	}

	stats := TransactionStatsToHttp(statsRaw)

	return iz.Respond().Status(200).JSON(stats)

}

func (api *Api) GetFilteredIncomeCategoriesHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	params := r.URL.Query()

	filter, err := IncomeCategoryCheckParams(params)
	if err != nil {
		return RespondError(err)
	}

	categories, err := api.Service.GetFilteredIncomeCategories(ctx, userId, filter)

	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered income categories | Error: %v", traceID, err)
		return RespondError(err)
	}

	var categoryList ListIncomeCategories
	categoryList.Categories = make([]IncomeCategoryResponseItem, 0, len(categories))
	for _, c := range categories {
		categoryList.Categories = append(categoryList.Categories, IncomeCategoryToHttp(c))
	}

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) GetFilteredExpenseCategoriesHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	params := r.URL.Query()

	filter, err := ExpenseCategoryCheckParams(params)
	if err != nil {
		return RespondError(err)
	}

	categories, err := api.Service.GetFilteredExpenseCategories(ctx, userId, filter)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered expense categories | Error: %v", traceID, err)
		return RespondError(err)
	}

	var categoryList ListExpenseCategories
	categoryList.Categories = make([]ExpenseCategoryResponseItem, 0, len(categories))
	for _, c := range categories {
		categoryList.Categories = append(categoryList.Categories, ExpenseCategoryToHttp(c))
	}

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) GetFilteredTransactionsHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	params := r.URL.Query()

	filter, err := TransactionCheckParams(params)
	if err != nil {
		return RespondError(err)
	}

	transactions, err := api.Service.GetFilteredTransactions(ctx, userId, filter)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get filtered transactions | Error: %v", traceID, err)
		return RespondError(err)
	}

	var transactionList ListTransactionResponse
	transactionList.Transactions = make([]TransactionItem, 0, len(transactions))

	for _, transaction := range transactions {
		transactionList.Transactions = append(transactionList.Transactions, TransactionToHttp(transaction))
	}

	return iz.Respond().Status(200).JSON(transactionList)
}

func (api *Api) GetTransactionByIdHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	txnId := r.PathValue("id")
	txn, err := api.Service.GetTranscationById(ctx, userId, txnId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to get transaction by id | Error: %v", traceID, err)
		return RespondError(err)
	}

	var transactionList ListTransactionResponse
	transactionList.Transactions = make([]TransactionItem, 0, 1)
	transactionList.Transactions = append(transactionList.Transactions, TransactionToHttp(txn))

	return iz.Respond().Status(200).JSON(transactionList)
}

func (api *Api) UpdateExpenseCategoryHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var updateExpCategoryReq UpdateExpenseCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&updateExpCategoryReq); err != nil {
		return iz.Respond().Status(401).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Invalid request body: %v", err.Error()),
		})
	}

	if updateExpCategoryReq.NewName == "" {
		return iz.Respond().Status(401).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "New category name cannot be empty!",
		})
	}

	updateExpCategoryItem := budget.UpdateExpenseCategoryRequest{
		ID:           updateExpCategoryReq.ID,
		NewName:      updateExpCategoryReq.NewName,
		NewMaxAmount: updateExpCategoryReq.NewMaxAmount,
		NewPeriodDay: updateExpCategoryReq.NewPeriodDay,
		NewNote:      updateExpCategoryReq.NewNote,
	}

	updatedCategory, err := api.Service.UpdateExpenseCategory(ctx, userId, updateExpCategoryItem)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to update expense category | Error: %v", traceID, err)
		return RespondError(err)
	}

	var categoryList ListExpenseCategories
	categoryList.Categories = make([]ExpenseCategoryResponseItem, 0, 1)
	categoryList.Categories = append(categoryList.Categories, ExpenseCategoryToHttp(*updatedCategory))

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) DeleteExpenseCategoryHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var categoryId string = r.PathValue("id")
	if categoryId == "" {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category ID is empty!",
		})
	}

	if err := api.Service.DeleteExpenseCategory(ctx, userId, categoryId); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to delete expense category | Error: %v", traceID, err)
		return RespondError(err)
	}

	return iz.Respond().Status(200).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Category deleted successfully.",
	})
}

func (api *Api) UpdateIncomeCategoryHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var updateIncCategoryReq UpdateIncomeCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&updateIncCategoryReq); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Invalid request body: %v", err.Error()),
		})
	}

	if updateIncCategoryReq.NewName == "" {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "New category name cannot be empty!",
		})
	}

	updateIncCategoryItem := budget.UpdateIncomeCategoryRequest{
		ID:              updateIncCategoryReq.ID,
		NewName:         updateIncCategoryReq.NewName,
		NewTargetAmount: updateIncCategoryReq.NewTargetAmount,
		NewNote:         updateIncCategoryReq.NewNote,
	}

	updatedCategory, err := api.Service.UpdateIncomeCategory(ctx, userId, updateIncCategoryItem)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to update income category | Error: %v", traceID, err)
		return RespondError(err)
	}

	var categoryList ListIncomeCategories
	categoryList.Categories = make([]IncomeCategoryResponseItem, 0, 1)
	categoryList.Categories = append(categoryList.Categories, IncomeCategoryToHttp(*updatedCategory))

	return iz.Respond().Status(200).JSON(categoryList)
}

func (api *Api) DeleteIncomeCategoryHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var categoryId string = r.PathValue("id")
	if categoryId == "" {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: "Category ID is empty!",
		})
	}

	if err := api.Service.DeleteIncomeCategory(ctx, userId, categoryId); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | failed to delete income category | Error: %v", traceID, err)
		return RespondError(err)
	}

	return iz.Respond().Status(200).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Category deleted successfully.",
	})
}

func (api *Api) LoginUserHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	var loginRequest UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginRequest); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Invalid request body: %v", err.Error()),
		})
	}

	credentials := auth.UserCredentialsPure{
		UserName:      loginRequest.UserName,
		PasswordPlain: loginRequest.Password,
	}

	token, err := api.Service.GenerateSession(ctx, credentials)
	if err != nil {
		return RespondError(err)
	}

	return iz.Respond().Status(200).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Welcome",
		Extra:   token,
	})
}

func (api *Api) LogoutUserHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	token := r.Header.Get("Authorization")
	if token == "" {
		return iz.Respond().Status(401).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Authorization header is required.",
		})
	}

	userId, err := api.Service.CheckSession(ctx, token)
	if err != nil {
		return RespondError(err)
	}

	if err := api.Service.LogoutUser(ctx, userId, token); err != nil {
		return RespondError(err)
	}

	return iz.Respond().Status(200).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Bye",
	})
}

func (api *Api) DownloadUserData(w http.ResponseWriter, r *http.Request) {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	token := r.Header.Get("Authorization")
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		errJson := appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Authorization header is required.",
		}
		if err := json.NewEncoder(w).Encode(errJson); err != nil {
			logging.Logger.Errorf("[TraceID=%s] Failed to encode error json: %v", traceID, err)
		}
		return
	}

	userId, err := api.Service.CheckSession(ctx, token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		errJson := appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Your token is not valid, please log in again",
		}
		if err := json.NewEncoder(w).Encode(errJson); err != nil {
			logging.Logger.Errorf("[TraceID=%s] Failed to encode error json: %v", traceID, err)
		}
		return
	}

	data, err := api.Service.DownloadUserData(ctx, userId)
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | Failed to get user data: %v", traceID, err)
		http.Error(w, "Failed to get user data", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	writeJSON := func(filename string, payload interface{}) error {
		file, err := zipWriter.Create(filename)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | Failed to create ZIP file %s: %v", traceID, filename, err)
			return err
		}
		jsonBytes, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | Failed to marshal JSON for %s: %v", traceID, filename, err)
			return err
		}
		_, err = file.Write(jsonBytes)
		if err != nil {
			logging.Logger.Errorf("[TraceID=%s] | Failed to write JSON to ZIP for %s: %v", traceID, filename, err)
		}
		return err
	}

	if err := writeJSON("transactions.json", data.Transactions); err != nil {
		http.Error(w, "Failed to write transactions", http.StatusInternalServerError)
		return
	}
	if err := writeJSON("expense_categories.json", data.ExpenseCategories); err != nil {
		http.Error(w, "Failed to write expense categories", http.StatusInternalServerError)
		return
	}
	if err := writeJSON("income_categories.json", data.IncomeCategories); err != nil {
		http.Error(w, "Failed to write income categories", http.StatusInternalServerError)
		return
	}

	if err := zipWriter.Close(); err != nil {
		logging.Logger.Errorf("[TraceID=%s] | Failed to finalize ZIP: %v", traceID, err)
		http.Error(w, "Failed to finalize ZIP file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="budget_tracker_my_data.zip"`)
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(buf.Bytes())
	if err != nil {
		logging.Logger.Errorf("[TraceID=%s] | Failed to write ZIP to response: %v", traceID, err)
	}
}

func (api *Api) CheckToken(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	token := r.Header.Get("Authorization")
	if token == "" {
		return iz.Respond().Status(401).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Authorization header is required.",
		})
	}

	validToken, err := api.Service.CheckSession(ctx, token)
	if err != nil {
		return RespondError(err)
	}

	if validToken == "" {
		return iz.Respond().Status(401).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "Authorization failed.",
		})
	}

	return iz.Respond().Status(200).Text(validToken)
}

func (api *Api) DeleteUserHandler(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	var deleteReqRaw DeleteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteReqRaw); err != nil {
		return iz.Respond().Status(400).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrInvalidInput,
			Message: fmt.Sprintf("Invalid request body: %v", err.Error()),
		})
	}

	deleteReq := auth.DeleteUser{
		Password: deleteReqRaw.Password,
		Reason:   deleteReqRaw.Reason,
	}

	if err := api.Service.DeleteUser(ctx, userId, deleteReq); err != nil {
		return RespondError(err)
	}

	return iz.Respond().Status(200).JSON(OperationResponse{
		Code:    SUCCESS_CODE,
		Message: "Account deleted successfully.",
	})
}

func (api *Api) GetAccountInfo(r *iz.Request) iz.Responder {
	traceID := uuid.NewString()
	ctx := context.WithValue(r.Context(), contextutil.TraceIDKey, traceID)

	userId, ok := r.Context().Value(userIdKey).(string)
	if !ok {
		return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
			Code:    appErrors.ErrAuth,
			Message: "UserID not found",
		})
	}

	accInfoRaw, err := api.Service.GetAccountInfo(ctx, userId)
	if err != nil {
		return RespondError(err)
	}
	accInfo := AccountInfoToHttp(accInfoRaw)

	return iz.Respond().Status(200).JSON(accInfo)
}

func RespondError(err error) iz.Responder {
	var errResp appErrors.ErrorResponse
	if errors.As(err, &errResp) {
		return iz.Respond().Status(HttpStatusFromErrorCode(errResp.Code)).JSON(errResp)
	}

	return iz.Respond().Status(500).JSON(appErrors.ErrorResponse{
		Code:    "UNKNOWN",
		Message: "UNEXPECTED ERROR",
	})

}
