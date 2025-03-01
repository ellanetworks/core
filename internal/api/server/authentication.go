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

type Role string

const (
	AdminRole          Role = "admin"
	ReadOnlyRole       Role = "readonly"
	NetworkManagerRole Role = "network-manager"
)

const AuthenticationAction = "user_authentication"

type claims struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Role  Role   `json:"role"`
	jwt.StandardClaims
}

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
				c.ClientIP(),
				"Unauthorized access attempt",
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("userID", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// Require checks if the user has the required role to access the resource
// The user will be allowed to access the resource if their role is in the allowedRoles list
func Require(allowedRoles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleIfc, exists := c.Get("role")
		if !exists {
			writeError(c, http.StatusForbidden, "Role not found")
			c.Abort()
			return
		}
		role, ok := roleIfc.(Role)
		if !ok {
			writeError(c, http.StatusForbidden, "Role not found")
			c.Abort()
			return
		}

		allowed := false
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				allowed = true
				break
			}
		}

		if !allowed {
			writeError(c, http.StatusForbidden, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequireAdminOrFirstUser(db *db.Database, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		numUsers, err := db.NumUsers()
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Internal Error")
			c.Abort()
			return
		}

		if numUsers > 0 {
			claims, err := getClaimsFromAuthorizationHeader(c.GetHeader("Authorization"), jwtSecret)
			if err != nil {
				logger.LogAuditEvent(
					AuthenticationAction,
					"",
					c.ClientIP(),
					"Unauthorized access attempt",
				)
				writeError(c, http.StatusUnauthorized, "Unauthorized access attempt")
				c.Abort()
				return
			}

			c.Set("userID", claims.ID)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)

			if claims.Role != AdminRole {
				writeError(c, http.StatusForbidden, "Admin role required")
				c.Abort()
				return
			}
		}
		c.Next()
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
