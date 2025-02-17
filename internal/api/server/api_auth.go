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
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type LookupTokenResponse struct {
	Valid bool `json:"valid"`
}

const (
	LoginAction       = "auth_login"
	LookupTokenAction = "auth_lookup_token" // #nosec G101
)

// Map between db role and jwt role
var roleMap = map[db.Role]Role{
	db.AdminRole:    AdminRole,
	db.ReadOnlyRole: ReadOnlyRole,
}

func Login(dbInstance *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginParams LoginParams
		err := c.ShouldBindJSON(&loginParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if loginParams.Email == "" {
			writeError(c.Writer, http.StatusBadRequest, "Email is required")
			return
		}
		if loginParams.Password == "" {
			writeError(c.Writer, http.StatusBadRequest, "Password is required")
			return
		}
		user, err := dbInstance.GetUser(loginParams.Email)
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				loginParams.Email,
				"User failed to log in",
			)
			writeError(c.Writer, http.StatusUnauthorized, "The email or password is incorrect. Try again.")
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(loginParams.Password))
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				user.Email,
				"User failed to log in",
			)
			writeError(c.Writer, http.StatusUnauthorized, "The email or password is incorrect. Try again.")
			return
		}

		role, ok := roleMap[db.Role(user.Role)]
		if !ok {
			writeError(c.Writer, http.StatusInternalServerError, "Internal Error")
			return
		}

		token, err := generateJWT(user.ID, user.Email, role, jwtSecret)
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
			user.Email,
			"User logged in",
		)
	}
}

func LookupToken(dbInstance *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			writeError(c.Writer, http.StatusBadRequest, "Authorization header is required")
			return
		}
		_, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		var valid bool
		if err != nil {
			valid = false
		} else {
			valid = true
		}
		lookupTokenResponse := LookupTokenResponse{
			Valid: valid,
		}
		err = writeResponse(c.Writer, lookupTokenResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal Error")
			return
		}
		logger.LogAuditEvent(
			LookupTokenAction,
			"",
			"User looked up token",
		)
	}
}
