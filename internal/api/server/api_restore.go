package server

import (
	"io"
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const RestoreAction = "restore_database"

func Restore(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value("email")
		emailStr, ok := email.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		err := r.ParseMultipartForm(32 << 20) // 32MB max memory buffer
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid multipart form", err, logger.APILog)
			return
		}

		file, _, err := r.FormFile("backup")
		if err != nil {
			writeError(w, http.StatusBadRequest, "No backup file provided", err, logger.APILog)
			return
		}
		defer file.Close()

		tempFile, err := os.CreateTemp("", "restore_*.db")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create temporary file", err, logger.APILog)
			return
		}
		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}()

		if _, err := io.Copy(tempFile, file); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to copy uploaded file", err, logger.APILog)
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to reset file pointer", err, logger.APILog)
			return
		}

		if err := dbInstance.Restore(tempFile); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to restore database", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Database restored successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			RestoreAction,
			emailStr,
			getClientIP(r),
			"User restored database",
		)
	}
}
