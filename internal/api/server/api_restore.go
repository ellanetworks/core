package server

import (
	"io"
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const RestoreAction = "restore_database"

func Restore(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}

		file, err := c.FormFile("backup")
		if err != nil {
			writeError(c, http.StatusBadRequest, "No backup file provided")
			return
		}

		tempFile, err := os.CreateTemp("", "restore_*.db")
		if err != nil {
			writeError(c, http.StatusInternalServerError, "failed to create temporary file")
			return
		}
		defer func() {
			err := tempFile.Close()
			if err != nil {
				logger.APILog.Warn("Failed to close temp restore file", zap.Error(err))
			}
			err = os.Remove(tempFile.Name())
			if err != nil {
				logger.APILog.Warn("Failed to remove temp restore file", zap.Error(err))
			}
		}()

		uploadedFile, err := file.Open()
		if err != nil {
			writeError(c, http.StatusInternalServerError, "failed to open uploaded file")
			return
		}
		defer uploadedFile.Close()

		if _, err := io.Copy(tempFile, uploadedFile); err != nil {
			writeError(c, http.StatusInternalServerError, "failed to copy uploaded file")
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			writeError(c, http.StatusInternalServerError, "failed to reset file pointer")
			return
		}

		if err := dbInstance.Restore(tempFile); err != nil {
			writeError(c, http.StatusInternalServerError, "failed to restore database")
			return
		}

		successResponse := SuccessResponse{
			Message: "Database restored successfully",
		}
		writeResponse(c, successResponse, http.StatusOK)
		logger.LogAuditEvent(
			RestoreAction,
			email,
			c.ClientIP(),
			"User restored database",
		)
	}
}
