package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

var tracer = otel.Tracer("ella-core/api/authentication_middleware")

const AuthenticationAction = "user_authentication"

type claims struct {
	ID     int    `json:"id"`
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
	jwt.RegisteredClaims
}

type contextKey string

const (
	contextKeyUserID contextKey = "userID"
	contextKeyEmail  contextKey = "email"
	contextKeyRoleID contextKey = "roleID"
)

func isAPIToken(tok string) bool { return strings.HasPrefix(tok, "ellacore_") }

func parseAPIToken(presented string) (tokenID, secret string, ok bool) {
	if !strings.HasPrefix(presented, "ellacore_") {
		return "", "", false
	}
	rest := strings.TrimPrefix(presented, "ellacore_")
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// authenticateRequest validates the Authorization header (JWT or API token),
// and returns (userID, email, roleID) for authorization.
func authenticateRequest(r *http.Request, jwtSecret []byte, store *db.Database) (int, string, RoleID, error) {
	_, span := tracer.Start(r.Context(), "Authenticate",
		trace.WithAttributes(),
	)
	defer span.End()

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0, "", 0, errors.New("missing Authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return 0, "", 0, errors.New("invalid Authorization scheme")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return 0, "", 0, errors.New("empty token")
	}

	// API token path
	if isAPIToken(token) {
		tokenID, token, ok := parseAPIToken(token)
		if !ok {
			return 0, "", 0, errors.New("invalid API token format")
		}
		tok, err := store.GetAPITokenByTokenID(r.Context(), tokenID)
		if err != nil || tok == nil {
			return 0, "", 0, errors.New("invalid API token")
		}
		if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
			return 0, "", 0, errors.New("API token expired")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(tok.TokenHash), []byte(token)); err != nil {
			return 0, "", 0, errors.New("invalid API token")
		}
		u, err := store.GetUserByID(r.Context(), tok.UserID)
		if err != nil || u == nil {
			return 0, "", 0, errors.New("user not found")
		}
		return u.ID, u.Email, RoleID(u.RoleID), nil
	}

	// JWT path
	cl, err := getClaimsFromJWT(r.Context(), token, jwtSecret)
	if err != nil {
		return 0, "", 0, err
	}
	return cl.ID, cl.Email, cl.RoleID, nil
}

// putIdentity adds identity to context.
func putIdentity(ctx context.Context, id int, email string, role RoleID) context.Context {
	ctx = context.WithValue(ctx, contextKeyUserID, id)
	ctx = context.WithValue(ctx, contextKeyEmail, email)
	ctx = context.WithValue(ctx, contextKeyRoleID, role)
	return ctx
}

func Authenticate(jwtSecret []byte, store *db.Database, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, email, role, err := authenticateRequest(r, jwtSecret, store)
		if err != nil {
			logger.LogAuditEvent(AuthenticationAction, "", getClientIP(r), "Unauthorized: "+err.Error())
			writeError(w, http.StatusUnauthorized, "Invalid token", err, logger.APILog)
			return
		}
		ctx := putIdentity(r.Context(), uid, email, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getClaimsFromJWT(ctx context.Context, bearerToken string, jwtSecret []byte) (*claims, error) {
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
