package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

const LookupTokenAction = "lookup_token"

type LookupTokenResponse struct {
	Valid bool `json:"valid"`
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
