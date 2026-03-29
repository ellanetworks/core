package server

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const RotateSecretAction = "auth_rotate_secret" // #nosec: G101

type RotateSecretResponse struct {
	Message string `json:"message"`
}

func RotateSecret(dbInstance *db.Database, jwtSecret *JWTSecret) http.Handler { // #nosec: G101
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newSecret := make([]byte, 32)
		if _, err := rand.Read(newSecret); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to generate new secret", err, logger.APILog)
			return
		}

		if err := dbInstance.SetJWTSecret(r.Context(), newSecret); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to store new secret", err, logger.APILog)
			return
		}

		jwtSecret.Set(newSecret)

		if err := dbInstance.DeleteAllSessions(r.Context()); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Secret rotated but failed to delete sessions", err, logger.APILog)
			return
		}

		email, _ := r.Context().Value(contextKeyEmail).(string)

		logger.LogAuditEvent(
			r.Context(),
			RotateSecretAction,
			email,
			getClientIP(r),
			fmt.Sprintf("JWT secret rotated by %s — all user sessions invalidated", email),
		)

		writeResponse(r.Context(), w, RotateSecretResponse{Message: "Secret rotated successfully. All user sessions have been invalidated."}, http.StatusOK, logger.APILog)
	})
}
