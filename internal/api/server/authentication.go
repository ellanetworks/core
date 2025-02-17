package server

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type Role int

const (
	AdminRole    Role = 0
	ReadOnlyRole Role = 1
)

const AuthenticationAction = "user_authentication"

type claims struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Role  Role   `json:"role"`
	jwt.StandardClaims
}

// Authenticate is a middleware that validates the JWT and populates the context
func Authenticate(jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header not found"})
			return
		}

		claims, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		if err != nil {
			logger.LogAuditEvent(
				AuthenticationAction,
				"",
				"Unauthorized access attempt",
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		// Set the necessary values in the context
		c.Set("userID", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleIfc, exists := c.Get("role")
		if !exists {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		role, ok := roleIfc.(Role)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if role != AdminRole {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func UserOrFirstUser(handlerFunc gin.HandlerFunc, db *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Replace this with actual logic to determine if this is the first user
		numUsers, err := db.NumUsers()
		if err != nil {
			logger.NmsLog.Warnf("Failed to retrieve number of users: %v", err)
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve users")
			return
		}

		if numUsers > 0 {
			claims, err := getClaimsFromAuthorizationHeader(c.GetHeader("Authorization"), jwtSecret)
			if err != nil {
				logger.LogAuditEvent(
					AuthenticationAction,
					"",
					"Unauthorized access attempt",
				)
				writeError(c.Writer, http.StatusUnauthorized, "Unauthorized")
				return
			}

			c.Set("userID", claims.ID)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)
		}
		handlerFunc(c)
	}
}

func RequirePermission(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Assume permissions have been set by a prior auth middleware
		permissionsIfc, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		permissions, ok := permissionsIfc.([]string)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		for _, perm := range permissions {
			if perm == requiredPermission {
				c.Next()
				return
			}
		}
		c.AbortWithStatus(http.StatusForbidden)
	}
}

func GenerateJWTSecret() ([]byte, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return bytes, fmt.Errorf("failed to generate JWT secret: %w", err)
	}
	return bytes, nil
}

// Helper function to generate a JWT
func generateJWT(id int, email string, role Role, jwtSecret []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		ID:    id,
		Email: email,
		Role:  role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireAfter(),
		},
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
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
