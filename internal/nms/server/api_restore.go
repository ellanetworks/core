package server

import (
	"io"
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
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
			writeError(c.Writer, http.StatusBadRequest, "No backup file provided")
			return
		}

		tempFile, err := os.CreateTemp("", "restore_*.db")
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to create temporary file")
			return
		}
		defer func() {
			err := tempFile.Close()
			if err != nil {
				logger.NmsLog.Warnf("Failed to close temp restore file: %v", err)
			}
			err = os.Remove(tempFile.Name())
			if err != nil {
				logger.NmsLog.Warnf("Failed to remove temp restore file: %v", err)
			}
		}()

		uploadedFile, err := file.Open()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to open uploaded file")
			return
		}
		defer uploadedFile.Close()

		if _, err := io.Copy(tempFile, uploadedFile); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to copy uploaded file")
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to reset file pointer")
			return
		}

		if err := dbInstance.Restore(tempFile); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to restore database")
			return
		}

		successResponse := SuccessResponse{
			Message: "Database restored successfully",
		}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}

		logger.LogAuditEvent(
			RestoreAction,
			email,
			"User restored database",
		)
	}
}
