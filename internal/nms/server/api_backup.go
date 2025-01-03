package server

import (
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

const BackupAction = "backup_database"

func Backup(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}

		backupFilePath, err := dbInstance.Backup()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		defer func() {
			if err := os.Remove(backupFilePath); err != nil {
				logger.NmsLog.Errorf("Failed to remove backup file: %v", err)
			}
		}()

		c.File(backupFilePath)
		logger.LogAuditEvent(
			BackupAction,
			email,
			"Successfully backed up database",
		)
	}
}
