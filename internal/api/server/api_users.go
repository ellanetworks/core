package server

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
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

// roleDBMap maps the role string to the db.Role enum.
var roleDBMap = map[string]db.Role{
	"admin":           db.AdminRole,
	"readonly":        db.ReadOnlyRole,
	"network-manager": db.NetworkManagerRole,
}

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

func ListUsers(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbUsers, err := dbInstance.ListUsers(c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to query users", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Unable to retrieve users")
			return
		}

		users := make([]GetUserParams, 0)
		for _, user := range dbUsers {
			users = append(users, GetUserParams{
				Email: user.Email,
				Role:  user.Role.String(),
			})
		}
		writeResponse(c, users, http.StatusOK)
		logger.LogAuditEvent(
			ListUsersAction,
			email,
			c.ClientIP(),
			"Successfully retrieved list of users",
		)
	}
}

func GetUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		emailParam := c.Param("email")
		if emailParam == "" {
			writeError(c, http.StatusBadRequest, "Missing email parameter")
			return
		}
		dbUser, err := dbInstance.GetUser(emailParam, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "User not found")
			return
		}

		user := GetUserParams{
			Email: dbUser.Email,
			Role:  dbUser.Role.String(),
		}
		writeResponse(c, user, http.StatusOK)
		logger.LogAuditEvent(
			GetUserAction,
			email,
			c.ClientIP(),
			"Successfully retrieved user",
		)
	}
}

func GetLoggedInUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, exists := c.Get("email")
		if !exists {
			writeError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		email, ok := emailAny.(string)
		if !ok {
			writeError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		dbUser, err := dbInstance.GetUser(email, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "User not found")
			return
		}

		user := GetUserParams{
			Email: dbUser.Email,
			Role:  dbUser.Role.String(),
		}
		writeResponse(c, user, http.StatusOK)
		logger.LogAuditEvent(
			GetLoggedInUserAction,
			email,
			c.ClientIP(),
			"Successfully retrieved logged in user",
		)
	}
}

func CreateUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			email = "First User"
		}
		var newUser CreateUserParams
		err := c.ShouldBindJSON(&newUser)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if newUser.Email == "" {
			writeError(c, http.StatusBadRequest, "email is missing")
			return
		}
		if newUser.Password == "" {
			writeError(c, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidEmail(newUser.Email) {
			writeError(c, http.StatusBadRequest, "Invalid email format")
			return
		}
		role, ok := roleDBMap[newUser.Role]
		if !ok {
			writeError(c, http.StatusBadRequest, "Invalid role")
			return
		}
		_, err = dbInstance.GetUser(newUser.Email, c.Request.Context())
		if err == nil {
			writeError(c, http.StatusBadRequest, "user already exists")
			return
		}
		hashedPassword, err := hashPassword(newUser.Password)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to hash password")
			return
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			Role:           role,
		}
		err = dbInstance.CreateUser(dbUser, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to create user", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to create user")
			return
		}
		successResponse := SuccessResponse{Message: "User created successfully"}
		writeResponse(c, successResponse, http.StatusCreated)
		logger.LogAuditEvent(
			CreateUserAction,
			email,
			c.ClientIP(),
			"User created user: "+newUser.Email+" with role: "+fmt.Sprint(newUser.Role),
		)
	}
}

func UpdateUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		emailParam := c.Param("email")
		if emailParam == "" {
			writeError(c, http.StatusBadRequest, "Missing email parameter")
			return
		}
		var updateUserParams UpdateUserParams
		err := c.ShouldBindJSON(&updateUserParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateUserParams.Email == "" {
			writeError(c, http.StatusBadRequest, "email is missing")
			return
		}
		if !isValidEmail(updateUserParams.Email) {
			writeError(c, http.StatusBadRequest, "Invalid email format")
			return
		}
		role, ok := roleDBMap[updateUserParams.Role]
		if !ok {
			writeError(c, http.StatusBadRequest, "Invalid role")
			return
		}
		_, err = dbInstance.GetUser(emailParam, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "User not found")
			return
		}
		err = dbInstance.UpdateUser(updateUserParams.Email, role, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update user", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update user")
			return
		}
		successResponse := SuccessResponse{Message: "User updated successfully"}
		writeResponse(c, successResponse, http.StatusOK)
		logger.LogAuditEvent(
			UpdateUserAction,
			email,
			c.ClientIP(),
			"User updated user: "+updateUserParams.Email,
		)
	}
}

func UpdateUserPassword(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		emailParam := c.Param("email")
		if emailParam == "" {
			writeError(c, http.StatusBadRequest, "Missing email parameter")
			return
		}
		var updateUserParams UpdateUserPasswordParams
		err := c.ShouldBindJSON(&updateUserParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateUserParams.Email == "" {
			writeError(c, http.StatusBadRequest, "email is missing")
			return
		}
		if updateUserParams.Password == "" {
			writeError(c, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidEmail(updateUserParams.Email) {
			writeError(c, http.StatusBadRequest, "Invalid email format")
			return
		}

		_, err = dbInstance.GetUser(emailParam, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "User not found")
			return
		}
		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			logger.APILog.Warn("Failed to hash password", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to hash password")
			return
		}
		err = dbInstance.UpdateUserPassword(updateUserParams.Email, hashedPassword, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update user password", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update user")
			return
		}
		successResponse := SuccessResponse{Message: "User password updated successfully"}
		writeResponse(c, successResponse, http.StatusOK)
		logger.LogAuditEvent(
			UpdateUserPasswordAction,
			email,
			c.ClientIP(),
			"User updated password for user: "+updateUserParams.Email,
		)
	}
}

func DeleteUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		emailParam := c.Param("email")
		if emailParam == "" {
			writeError(c, http.StatusBadRequest, "Missing email parameter")
			return
		}
		_, err := dbInstance.GetUser(emailParam, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "User not found")
			return
		}
		err = dbInstance.DeleteUser(emailParam, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to delete user", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to delete user")
			return
		}

		successResponse := SuccessResponse{Message: "User deleted successfully"}
		writeResponse(c, successResponse, http.StatusOK)
		logger.LogAuditEvent(
			DeleteUserAction,
			email,
			c.ClientIP(),
			"User deleted user: "+emailParam,
		)
	}
}
