package server

import (
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const RestoreAction = "restore_database"

func Restore(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		const maxRestoreSize = 256 << 20 // 256MB

		r.Body = http.MaxBytesReader(w, r.Body, maxRestoreSize)

		err := r.ParseMultipartForm(maxRestoreSize)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid multipart form", err, logger.APILog)
			return
		}

		file, _, err := r.FormFile("backup")
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "No backup file provided", err, logger.APILog)
			return
		}

		defer func() {
			err := file.Close()
			if err != nil {
				logger.EllaLog.Error("could not close uploaded file", zap.Error(err))
			}
		}()

		tempFile, err := os.CreateTemp(dbInstance.Dir(), "restore_*.tar.gz")
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create temporary file", err, logger.APILog)
			return
		}

		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}()

		if _, err := io.Copy(tempFile, file); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to copy uploaded file", err, logger.APILog)
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to reset file pointer", err, logger.APILog)
			return
		}

		if err := dbInstance.Restore(r.Context(), tempFile); err != nil {
			if errors.Is(err, db.ErrRestoreInProgress) {
				writeError(r.Context(), w, http.StatusConflict, "A restore is already in progress", nil, logger.APILog)
				return
			}

			if errors.Is(err, db.ErrInvalidBackupFile) {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid backup file", err, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to restore database", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Database restored successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			r.Context(),
			RestoreAction,
			emailStr,
			getClientIP(r),
			"User restored database",
		)
	}
}
