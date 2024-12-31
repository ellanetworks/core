package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type CreateUserParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type GetUserParams struct {
	Username string `json:"username"`
}

const (
	ListUsersAction       = "list_users"
	GetUserAction         = "get_user"
	GetLoggedInUserAction = "get_logged_in_user"
	CreateUserAction      = "create_user"
	UpdateUserAction      = "update_user"
	DeleteUserAction      = "delete_user"
)

func isValidUsername(username string) bool {
	if username == "me" {
		return false
	}
	if len(username) <= 0 {
		return false
	}
	if len(username) > 255 {
		return false
	}
	return true
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
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
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
				Username: user.Username,
			})
		}
		err = writeResponse(c.Writer, users, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			ListUsersAction,
			username,
			"Successfully retrieved list of users",
		)
	}
}

func GetUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		usernameParam := c.Param("username")
		if usernameParam == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing username parameter")
			return
		}
		dbUser, err := dbInstance.GetUser(usernameParam)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}

		user := GetUserParams{
			Username: dbUser.Username,
		}
		err = writeResponse(c.Writer, user, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetUserAction,
			username,
			"Successfully retrieved user",
		)
	}
}

func GetLoggedInUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, exists := c.Get("username")
		if !exists {
			writeError(c.Writer, http.StatusUnauthorized, "Unauthorized")
			return
		}
		username, ok := usernameAny.(string)
		if !ok {
			writeError(c.Writer, http.StatusUnauthorized, "Unauthorized")
			return
		}
		dbUser, err := dbInstance.GetUser(username)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}

		user := GetUserParams{
			Username: dbUser.Username,
		}
		err = writeResponse(c.Writer, user, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetLoggedInUserAction,
			username,
			"Successfully retrieved logged in user",
		)
	}
}

func CreateUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		var newUser CreateUserParams
		err := c.ShouldBindJSON(&newUser)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if newUser.Username == "" {
			writeError(c.Writer, http.StatusBadRequest, "username is missing")
			return
		}
		if newUser.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidUsername(newUser.Username) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid username format. Must be less than 256 characters")
			return
		}
		_, err = dbInstance.GetUser(newUser.Username)
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
			Username:       newUser.Username,
			HashedPassword: hashedPassword,
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
			username,
			"Successfully created user",
		)
	}
}

func UpdateUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		usernameParam := c.Param("username")
		if usernameParam == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing username parameter")
			return
		}
		var updateUserParams CreateUserParams
		err := c.ShouldBindJSON(&updateUserParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateUserParams.Username == "" {
			writeError(c.Writer, http.StatusBadRequest, "username is missing")
			return
		}
		if updateUserParams.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "password is missing")
			return
		}
		if !isValidUsername(updateUserParams.Username) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid username format. Must be less than 256 characters")
			return
		}

		_, err = dbInstance.GetUser(usernameParam)
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
			Username:       updateUserParams.Username,
			HashedPassword: hashedPassword,
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
			username,
			"Successfully updated user",
		)
	}
}

func DeleteUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		usernameParam := c.Param("username")
		if usernameParam == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing username parameter")
			return
		}
		_, err := dbInstance.GetUser(usernameParam)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}
		err = dbInstance.DeleteUser(usernameParam)
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
			username,
			"Successfully deleted user",
		)
	}
}
