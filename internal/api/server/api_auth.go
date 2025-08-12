package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
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
	LoginAction = "auth_login"
)

// Helper function to generate a JWT
func generateJWT(id int, email string, roleID RoleID, jwtSecret []byte) (string, error) {
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

func Login(dbInstance *db.Database, jwtSecret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var loginParams LoginParams
		if err := json.NewDecoder(r.Body).Decode(&loginParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format", err, logger.APILog)
			return
		}

		if loginParams.Email == "" {
			writeError(w, http.StatusBadRequest, "Email is required", fmt.Errorf("email is missing"), logger.APILog)
			return
		}
		if loginParams.Password == "" {
			writeError(w, http.StatusBadRequest, "Password is required", fmt.Errorf("password is missing"), logger.APILog)
			return
		}

		user, err := dbInstance.GetUser(r.Context(), loginParams.Email)
		if err != nil {
			logger.LogAuditEvent(
				LoginAction,
				loginParams.Email,
				getClientIP(r),
				"User failed to log in",
			)
			writeError(w, http.StatusUnauthorized, "The email or password is incorrect. Try again.", err, logger.APILog)
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(loginParams.Password)) != nil {
			logger.LogAuditEvent(
				LoginAction,
				user.Email,
				getClientIP(r),
				"User failed to log in",
			)
			writeError(w, http.StatusUnauthorized, "The email or password is incorrect. Try again.", fmt.Errorf("password mismatch"), logger.APILog)
			return
		}

		token, err := generateJWT(user.ID, user.Email, RoleID(user.RoleID), jwtSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		resp := LoginResponse{Token: token}
		writeResponse(w, resp, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			LoginAction,
			user.Email,
			getClientIP(r),
			"User logged in",
		)
	})
}

func LookupToken(dbInstance *db.Database, jwtSecret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusBadRequest, "Authorization header is required", errors.New("missing Authorization header"), logger.APILog)
			return
		}

		_, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		valid := err == nil

		lookupTokenResponse := LookupTokenResponse{
			Valid: valid,
		}

		writeResponse(w, lookupTokenResponse, http.StatusOK, logger.APILog)
	})
}
