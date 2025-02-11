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

const AuthenticationAction = "user_authentication"

func Any(handlerFunc gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerFunc(c)
	}
}

func User(handlerFunc gin.HandlerFunc, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		claims, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		if err != nil {
			logger.LogAuditEvent(
				AuthenticationAction,
				"",
				"Unauthorized access attempt",
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("userID", claims.ID)
		c.Set("email", claims.Email)

		handlerFunc(c)
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
		}
		handlerFunc(c)
	}
}

type jwtNotaryClaims struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	jwt.StandardClaims
}

func GenerateJWTSecret() ([]byte, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return bytes, fmt.Errorf("failed to generate JWT secret: %w", err)
	}
	return bytes, nil
}

// Helper function to generate a JWT
func generateJWT(id int, email string, jwtSecret []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtNotaryClaims{
		ID:    id,
		Email: email,
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

func getClaimsFromAuthorizationHeader(header string, jwtSecret []byte) (*jwtNotaryClaims, error) {
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

func getClaimsFromJWT(bearerToken string, jwtSecret []byte) (*jwtNotaryClaims, error) {
	claims := jwtNotaryClaims{}
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
