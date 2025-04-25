package server

import (
	"net/http"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

		tempFile, err := os.CreateTemp("", "backup_*.db")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp backup file"})
			return
		}
		defer func() {
			err := tempFile.Close()
			if err != nil {
				logger.APILog.Warn("Failed to close temp backup file", zap.Error(err))
			}
			err = os.Remove(tempFile.Name())
			if err != nil {
				logger.APILog.Warn("Failed to remove temp backup file", zap.Error(err))
			}
		}()

		err = dbInstance.Backup(tempFile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset file pointer"})
			return
		}

		c.FileAttachment(tempFile.Name(), "database_backup_"+time.Now().Format("20060102_150405")+".db")

		logger.LogAuditEvent(
			BackupAction,
			email,
			c.ClientIP(),
			"Successfully backed up database",
		)
	}
}
