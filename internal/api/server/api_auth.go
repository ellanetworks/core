package server

import (
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const TokenExpirationTime = time.Hour * 1

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

// Helper function to generate a JWT
func generateJWT(id int, email string, roleID int, jwtSecret []byte) (string, error) {
	expiresAt := jwt.NewNumericDate(time.Now().Add(TokenExpirationTime))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		ID:     id,
		Email:  email,
		RoleID: roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expiresAt,
		},
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func Login(dbInstance *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginParams LoginParams
		err := c.ShouldBindJSON(&loginParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if loginParams.Email == "" {
			writeError(c, http.StatusBadRequest, "Email is required")
			return
		}
		if loginParams.Password == "" {
			writeError(c, http.StatusBadRequest, "Password is required")
			return
		}
		user, err := dbInstance.GetUser(loginParams.Email, c.Request.Context())
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				loginParams.Email,
				c.ClientIP(),
				"User failed to log in",
			)
			writeError(c, http.StatusUnauthorized, "The email or password is incorrect. Try again.")
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(loginParams.Password))
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				user.Email,
				c.ClientIP(),
				"User failed to log in",
			)
			writeError(c, http.StatusUnauthorized, "The email or password is incorrect. Try again.")
			return
		}

		token, err := generateJWT(user.ID, user.Email, user.RoleID, jwtSecret)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Internal Error")
			return
		}

		loginResponse := LoginResponse{
			Token: token,
		}
		writeResponse(c, loginResponse, http.StatusOK)
		logger.LogAuditEvent(
			LoginAction,
			user.Email,
			c.ClientIP(),
			"User logged in",
		)
	}
}

func LookupToken(dbInstance *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			writeError(c, http.StatusBadRequest, "Authorization header is required")
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
		writeResponse(c, lookupTokenResponse, http.StatusOK)
		logger.LogAuditEvent(
			LookupTokenAction,
			"",
			c.ClientIP(),
			"User looked up token",
		)
	}
}
