package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type CreateUserParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   RoleID `json:"role_id"`
}

type UpdateUserParams struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

type UpdateUserPasswordParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateMyUserPasswordParams struct {
	Password string `json:"password"`
}

type User struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

type ListUsersResponse struct {
	Items      []User `json:"items"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalCount int    `json:"total_count"`
}

type APIToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type CreateAPITokenParams struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type CreateAPITokenResponse struct {
	Token string `json:"token"`
}

type ListAPITokensResponse struct {
	Items      []APIToken `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

const (
	CreateUserAction         = "create_user"
	UpdateUserAction         = "update_user"
	DeleteUserAction         = "delete_user"
	UpdateUserPasswordAction = "update_user_password"
	CreateAPITokenAction     = "create_api_token"
	DeleteAPITokenAction     = "delete_api_token"
)

const (
	MaxNumUsers     = 50
	MaxNumAPITokens = 12
)

const lettersAndDigits = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func hashPassword(password string) (string, error) {
	pw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(pw), nil
}

func ListUsers(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		dbUsers, total, err := dbInstance.ListUsersPage(r.Context(), page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Unable to retrieve users", err, logger.APILog)
			return
		}

		items := make([]User, 0, len(dbUsers))

		for _, user := range dbUsers {
			items = append(items, User{
				Email:  user.Email,
				RoleID: RoleID(user.RoleID),
			})
		}

		users := ListUsersResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, users, http.StatusOK, logger.APILog)
	})
}

func GetUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailParam := r.PathValue("email")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		dbUser, err := dbInstance.GetUser(r.Context(), emailParam)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "User not found", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to retrieve user", err, logger.APILog)
			return
		}

		resp := User{
			Email:  dbUser.Email,
			RoleID: RoleID(dbUser.RoleID),
		}

		writeResponse(w, resp, http.StatusOK, logger.APILog)
	})
}

func GetLoggedInUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized", errors.New("email missing in context"), logger.APILog)
			return
		}

		dbUser, err := dbInstance.GetUser(r.Context(), email)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "User not found", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to retrieve user", err, logger.APILog)
			return
		}

		user := User{
			Email:  dbUser.Email,
			RoleID: RoleID(dbUser.RoleID),
		}

		writeResponse(w, user, http.StatusOK, logger.APILog)
	})
}

func CreateUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, _ := emailAny.(string)

		var newUser CreateUserParams

		if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if newUser.Email == "" {
			writeError(w, http.StatusBadRequest, "email is missing", errors.New("missing email"), logger.APILog)
			return
		}

		if newUser.Password == "" {
			writeError(w, http.StatusBadRequest, "password is missing", errors.New("missing password"), logger.APILog)
			return
		}

		if !isValidEmail(newUser.Email) {
			writeError(w, http.StatusBadRequest, "Invalid email format", errors.New("bad format"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(newUser.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		numUsers, err := dbInstance.CountUsers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count users", err, logger.APILog)
			return
		}

		if numUsers >= MaxNumUsers {
			writeError(w, http.StatusBadRequest, "Maximum number of users reached ("+strconv.Itoa(MaxNumUsers)+")", nil, logger.APILog)
			return
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			RoleID:         db.RoleID(newUser.RoleID),
		}

		_, err = dbInstance.CreateUser(r.Context(), dbUser)
		if err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(w, http.StatusBadRequest, "User already exists", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to create user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			CreateUserAction,
			email,
			getClientIP(r),
			fmt.Sprintf("User created user: %s with role: %d", newUser.Email, newUser.RoleID),
		)
	})
}

func UpdateUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		requester, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := r.PathValue("email")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		var updateUserParams UpdateUserParams

		err := json.NewDecoder(r.Body).Decode(&updateUserParams)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if updateUserParams.Email == "" || !isValidEmail(updateUserParams.Email) {
			writeError(w, http.StatusBadRequest, "Invalid or missing email", errors.New("bad format"), logger.APILog)
			return
		}

		err = dbInstance.UpdateUser(r.Context(), updateUserParams.Email, db.RoleID(updateUserParams.RoleID))
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "User not found", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to update user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateUserAction, requester, getClientIP(r), "User updated user: "+updateUserParams.Email)
	})
}

func UpdateUserPassword(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		requester, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := r.PathValue("email")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		var updateUserParams UpdateUserPasswordParams

		if err := json.NewDecoder(r.Body).Decode(&updateUserParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if updateUserParams.Email == "" || updateUserParams.Password == "" || !isValidEmail(updateUserParams.Email) {
			writeError(w, http.StatusBadRequest, "Invalid input", errors.New("bad input"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		err = dbInstance.UpdateUserPassword(r.Context(), updateUserParams.Email, hashedPassword)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "User not found", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to update password", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User password updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateUserPasswordAction, requester, getClientIP(r), "User updated password for user: "+updateUserParams.Email)
	})
}

func UpdateMyUserPassword(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized", errors.New("email missing in context"), logger.APILog)
			return
		}

		var updateUserParams UpdateMyUserPasswordParams

		err := json.NewDecoder(r.Body).Decode(&updateUserParams)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if updateUserParams.Password == "" {
			writeError(w, http.StatusBadRequest, "password is missing", errors.New("missing password"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		err = dbInstance.UpdateUserPassword(r.Context(), email, hashedPassword)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update password", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User password updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateUserPasswordAction, email, getClientIP(r), "User updated own password")
	})
}

func DeleteUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		requester, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := r.PathValue("email")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		err := dbInstance.DeleteUser(r.Context(), emailParam)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "User not found", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to delete user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteUserAction, requester, getClientIP(r), "User deleted user: "+emailParam)
	})
}

func ListMyAPITokens(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized", errors.New("email missing in context"), logger.APILog)
			return
		}

		userIDAny := r.Context().Value(contextKeyUserID)

		userID, ok := userIDAny.(int64)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get user ID", errors.New("user ID missing in context"), logger.APILog)
			return
		}

		tokens, total, err := dbInstance.ListAPITokensPage(r.Context(), userID, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Unable to retrieve API tokens", err, logger.APILog)
			return
		}

		items := make([]APIToken, 0, len(tokens))
		for _, token := range tokens {
			var expiresAt string
			if token.ExpiresAt != nil {
				expiresAt = token.ExpiresAt.Format(time.RFC3339)
			}
			items = append(items, APIToken{
				ID:        token.TokenID,
				Name:      token.Name,
				ExpiresAt: expiresAt,
			})
		}

		response := ListAPITokensResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func randAlphaNum(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		x, err := rand.Int(rand.Reader, big.NewInt(int64(len(lettersAndDigits))))
		if err != nil {
			return "", err
		}
		b[i] = lettersAndDigits[x.Int64()]
	}
	return string(b), nil
}

func hashAPIToken(token string) string {
	hashedToken, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		logger.APILog.Error("Failed to hash API token", zap.Error(err))
		return ""
	}

	return string(hashedToken)
}

func CreateMyAPIToken(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized", errors.New("email missing in context"), logger.APILog)
			return
		}

		var params CreateAPITokenParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Name == "" {
			writeError(w, http.StatusBadRequest, "Token name is required", errors.New("missing token name"), logger.APILog)
			return
		}

		if len(params.Name) < 3 || len(params.Name) > 50 {
			writeError(w, http.StatusBadRequest, "Token name must be between 3 and 50 characters", errors.New("invalid token name length"), logger.APILog)
			return
		}

		userIDAny := r.Context().Value(contextKeyUserID)

		userID, ok := userIDAny.(int64)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get user ID", errors.New("user ID missing in context"), logger.APILog)
			return
		}

		var expiresAt *time.Time
		if params.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, params.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid expiration time format", err, logger.APILog)
				return
			}

			if t.Before(time.Now()) {
				writeError(w, http.StatusBadRequest, "Expiration time must be in the future", errors.New("invalid expiration time"), logger.APILog)
				return
			}

			expiresAt = &t
		}

		numTokens, err := dbInstance.CountAPITokens(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count API tokens", err, logger.APILog)
			return
		}

		if numTokens >= MaxNumAPITokens {
			writeError(w, http.StatusBadRequest, "Maximum number of API tokens reached ("+strconv.Itoa(MaxNumAPITokens)+")", nil, logger.APILog)
			return
		}

		tokenID, err := randAlphaNum(12)
		if err != nil {
			logger.APILog.Error("Failed to generate token ID", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to generate token ID", err, logger.APILog)
			return
		}

		secret, err := randAlphaNum(24)
		if err != nil {
			logger.APILog.Error("Failed to generate secret", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to generate secret", err, logger.APILog)
			return
		}

		token := fmt.Sprintf("ellacore_%s_%s", tokenID, secret)

		hash := hashAPIToken(secret)

		apiToken := &db.APIToken{
			TokenID:   tokenID,
			Name:      params.Name,
			UserID:    userID,
			TokenHash: hash,
			ExpiresAt: expiresAt,
		}

		err = dbInstance.CreateAPIToken(r.Context(), apiToken)
		if err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(w, http.StatusConflict, "API token already exists", nil, logger.APILog)
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to create API token", err, logger.APILog)
			return
		}

		response := CreateAPITokenResponse{
			Token: token,
		}

		writeResponse(w, response, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			CreateAPITokenAction,
			email,
			getClientIP(r),
			fmt.Sprintf("User created API token: %s", apiToken.Name),
		)
	})
}

func DeleteMyAPIToken(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusUnauthorized, "Unauthorized", errors.New("email missing in context"), logger.APILog)
			return
		}

		idParam := r.PathValue("id")
		if idParam == "" {
			writeError(w, http.StatusBadRequest, "Missing token ID parameter", errors.New("missing param"), logger.APILog)
			return
		}

		userIDAny := r.Context().Value(contextKeyUserID)

		userID, ok := userIDAny.(int64)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get user ID", errors.New("user ID missing in context"), logger.APILog)
			return
		}

		token, err := dbInstance.GetAPITokenByTokenID(r.Context(), idParam)
		if err != nil {
			writeError(w, http.StatusNotFound, "API token not found", nil, logger.APILog)
			return
		}

		if token.UserID != userID {
			writeError(w, http.StatusForbidden, "You do not have permission to delete this token", errors.New("forbidden"), logger.APILog)
			return
		}

		if err := dbInstance.DeleteAPIToken(r.Context(), token.ID); err != nil {
			logger.APILog.Warn("Failed to delete API token", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to delete API token", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "API token deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			DeleteAPITokenAction,
			email,
			getClientIP(r),
			fmt.Sprintf("User deleted API token: %s", token.Name),
		)
	})
}
