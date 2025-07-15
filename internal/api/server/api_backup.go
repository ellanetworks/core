package server

import (
	"net/http"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const BackupAction = "backup_database"

func Backup(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value("email")
		emailStr, ok := email.(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		tempFile, err := os.CreateTemp("", "backup_*.db")
		if err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to create temp backup file", err, logger.APILog)
			return
		}
		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}()

		if err := dbInstance.Backup(tempFile); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to backup database", err, logger.APILog)
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to reset file pointer", err, logger.APILog)
			return
		}

		w.Header().Set("Content-Disposition", "attachment; filename=\"database_backup_"+time.Now().Format("20060102_150405")+".db\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, "", time.Now(), tempFile)

		logger.LogAuditEvent(
			BackupAction,
			emailStr,
			getClientIP(r),
			"Successfully backed up database",
		)
	}
}
