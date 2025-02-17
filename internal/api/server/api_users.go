package server

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type CreateUserParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     int    `json:"role"`
}

type UpdateUserPasswordParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GetUserParams struct {
	Email string `json:"email"`
	Role  int    `json:"role"`
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

func ListUsers(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbUsers, err := dbInstance.ListUsers()
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve users")
			return
		}

		users := make([]GetUserParams, 0)
		for _, user := range dbUsers {
			users = append(users, GetUserParams{
				Email: user.Email,
				Role:  user.Role,
			})
		}
		err = writeResponse(c.Writer, users, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			ListUsersAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Missing email parameter")
			return
		}
		dbUser, err := dbInstance.GetUser(emailParam)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}

		user := GetUserParams{
			Email: dbUser.Email,
			Role:  dbUser.Role,
		}
		err = writeResponse(c.Writer, user, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetUserAction,
			email,
			"Successfully retrieved user",
		)
	}
}

func GetLoggedInUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, exists := c.Get("email")
		if !exists {
			writeError(c.Writer, http.StatusUnauthorized, "Unauthorized")
			return
		}
		email, ok := emailAny.(string)
		if !ok {
			writeError(c.Writer, http.StatusUnauthorized, "Unauthorized")
			return
		}
		dbUser, err := dbInstance.GetUser(email)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}

		user := GetUserParams{
			Email: dbUser.Email,
			Role:  dbUser.Role,
		}
		err = writeResponse(c.Writer, user, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetLoggedInUserAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if newUser.Email == "" {
			writeError(c.Writer, http.StatusBadRequest, "email is missing")
			return
		}
		if newUser.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidEmail(newUser.Email) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid email format")
			return
		}
		if newUser.Role < 0 || newUser.Role > 2 {
			writeError(c.Writer, http.StatusBadRequest, "Invalid role")
			return
		}
		_, err = dbInstance.GetUser(newUser.Email)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "user already exists")
			return
		}
		hashedPassword, err := hashPassword(newUser.Password)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to hash password")
			return
		}

		dbUser := &db.User{
			Email:          newUser.Email,
			HashedPassword: hashedPassword,
			Role:           newUser.Role,
		}
		err = dbInstance.CreateUser(dbUser)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create user")
			return
		}
		successResponse := SuccessResponse{Message: "User created successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			CreateUserAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Missing email parameter")
			return
		}
		var updateUserParams CreateUserParams
		err := c.ShouldBindJSON(&updateUserParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateUserParams.Email == "" {
			writeError(c.Writer, http.StatusBadRequest, "email is missing")
			return
		}
		if updateUserParams.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidEmail(updateUserParams.Email) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid email format")
			return
		}
		if updateUserParams.Role < 0 || updateUserParams.Role > 2 {
			writeError(c.Writer, http.StatusBadRequest, "Invalid role")
			return
		}
		_, err = dbInstance.GetUser(emailParam)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}
		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to hash password")
			return
		}
		dbUser := &db.User{
			Email:          updateUserParams.Email,
			HashedPassword: hashedPassword,
			Role:           updateUserParams.Role,
		}
		err = dbInstance.UpdateUser(dbUser)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update user")
			return
		}
		successResponse := SuccessResponse{Message: "User updated successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateUserAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Missing email parameter")
			return
		}
		var updateUserParams UpdateUserPasswordParams
		err := c.ShouldBindJSON(&updateUserParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateUserParams.Email == "" {
			writeError(c.Writer, http.StatusBadRequest, "email is missing")
			return
		}
		if updateUserParams.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidEmail(updateUserParams.Email) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid email format")
			return
		}

		_, err = dbInstance.GetUser(emailParam)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}
		hashedPassword, err := hashPassword(updateUserParams.Password)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to hash password")
			return
		}

		dbUser := &db.User{
			Email:          updateUserParams.Email,
			HashedPassword: hashedPassword,
		}
		err = dbInstance.UpdateUser(dbUser)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update user")
			return
		}
		successResponse := SuccessResponse{Message: "User password updated successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateUserPasswordAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Missing email parameter")
			return
		}
		_, err := dbInstance.GetUser(emailParam)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}
		err = dbInstance.DeleteUser(emailParam)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to delete user")
			return
		}

		successResponse := SuccessResponse{Message: "User deleted successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			DeleteUserAction,
			email,
			"User deleted user: "+emailParam,
		)
	}
}
