package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const AuthenticationAction = "user_authentication"

type claims struct {
	ID     int    `json:"id"`
	Email  string `json:"email"`
	RoleID int    `json:"role_id"`
	jwt.RegisteredClaims
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
			logger.LogAuditEvent(AuthenticationAction, "", c.ClientIP(), "Unauthorized access attempt")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("userID", claims.ID)
		c.Set("email", claims.Email)
		c.Set("role_id", claims.RoleID)

		c.Next()
	}
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
