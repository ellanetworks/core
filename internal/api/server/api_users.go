package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type CreateUserParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateUserParams struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UpdateUserPasswordParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GetUserParams struct {
	Email string `json:"email"`
	Role  string `json:"role"`
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
	// Regular expression for a valid email format.
	// This regex ensures a proper structure: local-part@domain.
	const emailRegex = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`

	// Compile the regex for reuse.
	re := regexp.MustCompile(emailRegex)

	// Check email length constraints.
	if len(email) == 0 || len(email) > 255 {
		return false
	}

	// Validate the email format using the regex.
	return re.MatchString(email)
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
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		dbUsers, err := dbInstance.ListUsers(r.Context())
		if err != nil {
			logger.APILog.Warn("Failed to query users", zap.Error(err))
			writeErrorHTTP(w, http.StatusInternalServerError, "Unable to retrieve users", err, logger.APILog)
			return
		}

		users := make([]GetUserParams, 0, len(dbUsers))
		for _, user := range dbUsers {
			users = append(users, GetUserParams{
				Email: user.Email,
				Role:  roleIDToName[user.RoleID],
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
		emailAny := r.Context().Value("email")
		requester, ok := emailAny.(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
		if emailParam == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		dbUser, err := dbInstance.GetUser(r.Context(), emailParam)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		resp := GetUserParams{
			Email: dbUser.Email,
			Role:  roleIDToName[dbUser.RoleID],
		}
		writeResponse(w, resp, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(GetUserAction, requester, getClientIP(r), "Successfully retrieved user")
	})
}

func GetLoggedInUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok || email == "" {
			writeErrorHTTP(w, http.StatusUnauthorized, "Unauthorized", errors.New("email missing in context"), logger.APILog)
			return
		}

		dbUser, err := dbInstance.GetUser(r.Context(), email)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		user := GetUserParams{
			Email: dbUser.Email,
			Role:  roleIDToName[dbUser.RoleID],
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
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok || email == "" {
			email = "First User"
		}

		var newUser CreateUserParams
		if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}
		if newUser.Email == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "email is missing", errors.New("missing email"), logger.APILog)
			return
		}
		if newUser.Password == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "password is missing", errors.New("missing password"), logger.APILog)
			return
		}
		if !isValidEmail(newUser.Email) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid email format", errors.New("bad format"), logger.APILog)
			return
		}
		if _, err := dbInstance.GetUser(r.Context(), newUser.Email); err == nil {
			writeErrorHTTP(w, http.StatusBadRequest, "user already exists", errors.New("duplicate"), logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(newUser.Password)
		if err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			RoleID:         roleNameToID[newUser.Role],
		}
		if err := dbInstance.CreateUser(r.Context(), dbUser); err != nil {
			logger.APILog.Warn("Failed to create user", zap.Error(err))
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to create user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			CreateUserAction,
			email,
			getClientIP(r),
			"User created user: "+newUser.Email+" with role: "+newUser.Role,
		)
	})
}

func UpdateUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		requester, ok := emailAny.(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
		if emailParam == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		var updateUserParams UpdateUserParams
		if err := json.NewDecoder(r.Body).Decode(&updateUserParams); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}
		if updateUserParams.Email == "" || !isValidEmail(updateUserParams.Email) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid or missing email", errors.New("bad format"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetUser(r.Context(), emailParam); err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateUser(r.Context(), updateUserParams.Email, roleNameToID[updateUserParams.Role]); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to update user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateUserAction, requester, getClientIP(r), "User updated user: "+updateUserParams.Email)
	})
}

func UpdateUserPassword(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		requester, ok := emailAny.(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/password"), "/api/v1/users/")
		if emailParam == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		var updateUserParams UpdateUserPasswordParams
		if err := json.NewDecoder(r.Body).Decode(&updateUserParams); err != nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}
		if updateUserParams.Email == "" || updateUserParams.Password == "" || !isValidEmail(updateUserParams.Email) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid input", errors.New("bad input"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetUser(r.Context(), emailParam); err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to hash password", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateUserPassword(r.Context(), updateUserParams.Email, hashedPassword); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to update password", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User password updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateUserPasswordAction, requester, getClientIP(r), "User updated password for user: "+updateUserParams.Email)
	})
}

func DeleteUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		requester, ok := emailAny.(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("email missing in context"), logger.APILog)
			return
		}

		emailParam := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
		if emailParam == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetUser(r.Context(), emailParam); err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "User not found", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteUser(r.Context(), emailParam); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to delete user", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "User deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeleteUserAction, requester, getClientIP(r), "User deleted user: "+emailParam)
	})
}
