package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	AccessTokenDuration    = 15 * time.Minute    // short-lived
	SessionTokenDuration   = 30 * 24 * time.Hour // long-lived
	SessionTokenCookieName = "session_token"
)

type LoginParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshResponse struct {
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
	expiresAt := jwt.NewNumericDate(time.Now().Add(AccessTokenDuration))
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

func Refresh(dbInstance *db.Database, jwtSecret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionTokenCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "No session token", err, logger.APILog)
			return
		}

		rawToken, err := base64.URLEncoding.DecodeString(cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid token encoding", err, logger.APILog)
			return
		}

		hashed := sha256.Sum256(rawToken)

		session, err := dbInstance.GetSessionByTokenHash(r.Context(), hashed[:])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid session token", err, logger.APILog)
			return
		}

		expiresAt := time.Unix(session.ExpiresAt, 0)

		if time.Now().After(expiresAt) {
			err = dbInstance.DeleteSessionByTokenHash(r.Context(), hashed[:])
			if err != nil {
				logger.APILog.Error("Error deleting expired session", zap.Error(err))
			}

			writeError(w, http.StatusUnauthorized, "Session expired", errors.New("session expired"), logger.APILog)

			return
		}

		user, err := dbInstance.GetUserByID(r.Context(), session.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		token, err := generateJWT(user.ID, user.Email, RoleID(user.RoleID), jwtSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		resp := RefreshResponse{Token: token}

		writeResponse(w, resp, http.StatusOK, logger.APILog)
	})
}

func Login(dbInstance *db.Database, secureCookie bool) http.Handler {
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

		err = createSessionAndSetCookie(r.Context(), dbInstance, user.ID, secureCookie, w)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal Error", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Login successful"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			LoginAction,
			user.Email,
			getClientIP(r),
			"User logged in successfully",
		)
	})
}

func createSessionAndSetCookie(ctx context.Context, dbInstance *db.Database, userID int, secureCookie bool, w http.ResponseWriter) error {
	rawToken := make([]byte, 32)

	_, err := rand.Read(rawToken)
	if err != nil {
		return fmt.Errorf("couldn't create random token: %w", err)
	}

	tokenHash := sha256.Sum256(rawToken)

	expiresAt := time.Now().Add(SessionTokenDuration)

	session := &db.Session{
		UserID:    userID,
		TokenHash: tokenHash[:],
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expiresAt.Unix(),
	}

	_, err = dbInstance.CreateSession(ctx, session)
	if err != nil {
		return fmt.Errorf("couldn't create session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionTokenCookieName,
		Value:    base64.URLEncoding.EncodeToString(rawToken),
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})

	return nil
}

func LookupToken(dbInstance *db.Database, jwtSecret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			writeError(w, http.StatusBadRequest, "Authorization header is required",
				errors.New("missing Authorization header"), logger.APILog)
			return
		}

		_, _, _, err := authenticateRequest(r, jwtSecret, dbInstance)
		lookupTokenResponse := LookupTokenResponse{Valid: err == nil}

		writeResponse(w, lookupTokenResponse, http.StatusOK, logger.APILog)
	})
}

func Logout(dbInstance *db.Database, secureCookie bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionTokenCookieName)
		if err == nil && cookie.Value != "" {
			if raw, decErr := base64.URLEncoding.DecodeString(cookie.Value); decErr == nil {
				hashed := sha256.Sum256(raw)

				err = dbInstance.DeleteSessionByTokenHash(r.Context(), hashed[:])
				if err != nil {
					logger.APILog.Error("Error deleting session during logout", zap.Error(err))
				}
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     SessionTokenCookieName,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secureCookie,
		})

		w.WriteHeader(http.StatusNoContent)
	})
}
