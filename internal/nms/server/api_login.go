package server

import (
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func expireAfter() int64 {
	return time.Now().Add(time.Hour * 1).Unix()
}

type LoginParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

const LoginAction = "login"

func Login(dbInstance *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginParams LoginParams
		err := c.ShouldBindJSON(&loginParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if loginParams.Username == "" {
			writeError(c.Writer, http.StatusBadRequest, "Username is required")
			return
		}
		if loginParams.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "Password is required")
			return
		}
		user, err := dbInstance.GetUser(loginParams.Username)
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				user.Username,
				"User failed to log in",
			)
			writeError(c.Writer, http.StatusUnauthorized, "The username or password is incorrect. Try again.")
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(loginParams.Password))
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				user.Username,
				"User failed to log in",
			)
			writeError(c.Writer, http.StatusUnauthorized, "The username or password is incorrect. Try again.")
			return
		}

		token, err := generateJWT(user.ID, user.Username, jwtSecret)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal Error")
			return
		}

		loginResponse := LoginResponse{
			Token: token,
		}
		err = writeResponse(c.Writer, loginResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal Error")
			return
		}
		logger.LogAuditEvent(
			LoginAction,
			user.Username,
			"User logged in",
		)
	}
}
