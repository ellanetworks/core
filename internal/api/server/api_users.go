package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

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

type GetUserParams struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

const (
	ListUsersAction          = "list_users"
	GetUserAction            = "get_user"
	GetLoggedInUserAction    = "get_logged_in_user"
	CreateUserAction         = "create_user"
	UpdateUserAction         = "update_user"
	DeleteUserAction         = "delete_user"
	UpdateUserPasswordAction = "update_user_password"
)

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
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		dbUsers, err := dbInstance.ListUsers(r.Context())
		if err != nil {
			logger.APILog.Warn("Failed to query users", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Unable to retrieve users", err, logger.APILog)
			return
		}

		users := make([]GetUserParams, 0, len(dbUsers))
		for _, user := range dbUsers {
			users = append(users, GetUserParams{
				Email:  user.Email,
				RoleID: RoleID(user.RoleID),
			})
		}

		writeResponse(w, users, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			ListUsersAction,
			email,
			getClientIP(r),
			"Successfully retrieved list of users",
		)
	})
}

func GetUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		requester, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		dbUser, err := dbInstance.GetUser(r.Context(), emailParam)
		if err != nil {
			writeError(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		resp := GetUserParams{
			Email:  dbUser.Email,
			RoleID: RoleID(dbUser.RoleID),
		}
		writeResponse(w, resp, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(GetUserAction, requester, getClientIP(r), "Successfully retrieved user")
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
			writeError(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		user := GetUserParams{
			Email:  dbUser.Email,
			RoleID: RoleID(dbUser.RoleID),
		}

		writeResponse(w, user, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			GetLoggedInUserAction,
			email,
			getClientIP(r),
			"Successfully retrieved logged in user",
		)
	})
}

func CreateUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok || email == "" {
			email = "First User"
		}

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
		if _, err := dbInstance.GetUser(r.Context(), newUser.Email); err == nil {
			writeError(w, http.StatusBadRequest, "user already exists", errors.New("duplicate"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(newUser.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		numUsers, err := dbInstance.NumUsers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count users", err, logger.APILog)
			return
		}
		if numUsers == 0 {
			if newUser.RoleID != RoleAdmin {
				writeError(w, http.StatusBadRequest, "First user must be an admin", errors.New("first user must be admin"), logger.APILog)
				return
			}
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			RoleID:         db.RoleID(newUser.RoleID),
		}
		if err := dbInstance.CreateUser(r.Context(), dbUser); err != nil {
			logger.APILog.Warn("Failed to create user", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to create user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
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

		emailParam := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		var updateUserParams UpdateUserParams
		if err := json.NewDecoder(r.Body).Decode(&updateUserParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}
		if updateUserParams.Email == "" || !isValidEmail(updateUserParams.Email) {
			writeError(w, http.StatusBadRequest, "Invalid or missing email", errors.New("bad format"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetUser(r.Context(), emailParam); err != nil {
			writeError(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateUser(r.Context(), updateUserParams.Email, db.RoleID(updateUserParams.RoleID)); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateUserAction, requester, getClientIP(r), "User updated user: "+updateUserParams.Email)
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

		emailParam := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/password"), "/api/v1/users/")
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

		if _, err := dbInstance.GetUser(r.Context(), emailParam); err != nil {
			writeError(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateUserPassword(r.Context(), updateUserParams.Email, hashedPassword); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update password", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User password updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateUserPasswordAction, requester, getClientIP(r), "User updated password for user: "+updateUserParams.Email)
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

		emailParam := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
		if emailParam == "" {
			writeError(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetUser(r.Context(), emailParam); err != nil {
			writeError(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteUser(r.Context(), emailParam); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to delete user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeleteUserAction, requester, getClientIP(r), "User deleted user: "+emailParam)
	})
}
