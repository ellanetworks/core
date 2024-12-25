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

func isValidUsername(username string) bool {
	return len(username) > 0 && len(username) < 256
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
		setCorsHeader(c)
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
	}
}

func GetUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		username := c.Param("username")
		if username == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing username parameter")
			return
		}
		logger.NmsLog.Infof("Received GET user %v", username)
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
	}
}

func CreateUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
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
		logger.NmsLog.Infof("created user %v", newUser.Username)
		successResponse := SuccessResponse{Message: "User created successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func UpdateUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		username := c.Param("username")
		if username == "" {
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

		_, err = dbInstance.GetUser(username)
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
		logger.NmsLog.Infof("updated user %v", username)
		successResponse := SuccessResponse{Message: "User updated successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteUser(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		username := c.Param("username")
		if username == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing username parameter")
			return
		}
		_, err := dbInstance.GetUser(username)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "User not found")
			return
		}
		err = dbInstance.DeleteUser(username)
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
	}
}
