package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/golang-jwt/jwt/v5"
)

const AuthenticationAction = "user_authentication"

type claims struct {
	ID     int    `json:"id"`
	Email  string `json:"email"`
	RoleID int    `json:"role_id"`
	jwt.RegisteredClaims
}

func Authenticate(jwtSecret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "Authorization header not found", errors.New("missing header"), logger.APILog)
			return
		}

		claims, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		if err != nil {
			logger.LogAuditEvent(AuthenticationAction, "", getClientIP(r), "Unauthorized access attempt")
			writeError(w, http.StatusUnauthorized, "Invalid token", err, logger.APILog)
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), "userID", claims.ID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		ctx = context.WithValue(ctx, "roleID", claims.RoleID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getClaimsFromAuthorizationHeader(header string, jwtSecret []byte) (*claims, error) {
	if header == "" {
		return nil, fmt.Errorf("authorization header not found")
	}
	bearerToken := strings.Split(header, " ")
	if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
		return nil, fmt.Errorf("authorization header couldn't be processed. The expected format is 'Bearer <token>'")
	}
	claims, err := getClaimsFromJWT(bearerToken[1], jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("token is not valid: %s", err)
	}
	return claims, nil
}

func getClaimsFromJWT(bearerToken string, jwtSecret []byte) (*claims, error) {
	claims := claims{}
	token, err := jwt.ParseWithClaims(bearerToken, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return &claims, nil
}
