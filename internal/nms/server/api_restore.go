package server

import (
	"fmt"
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

		tempFilePath := fmt.Sprintf("./uploads/%s", file.Filename)
		if err := os.MkdirAll("./uploads", os.ModePerm); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to create temporary directory")
			return
		}

		if err := c.SaveUploadedFile(file, tempFilePath); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to save uploaded file")
			return
		}

		if err := dbInstance.Restore(tempFilePath); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to restore database")
			return
		}

		if err := os.Remove(tempFilePath); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "failed to remove temporary file")
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
