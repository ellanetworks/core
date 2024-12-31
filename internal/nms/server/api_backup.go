package server

import (
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

func Backup(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
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
	}
}
