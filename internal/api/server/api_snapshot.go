package server

import (
	"io"
	"net/http"
	"os"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	SnapshotAction        = "raft_snapshot"
	SnapshotRestoreAction = "raft_snapshot_restore"
)

func CreateSnapshot(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		appliedIndex, err := dbInstance.RaftSnapshot()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create snapshot", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Snapshot created"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			r.Context(),
			SnapshotAction,
			emailStr,
			getClientIP(r),
			"Created Raft snapshot at index",
		)

		_ = appliedIndex
	})
}

func RestoreSnapshot(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		const maxSnapshotSize = 256 << 20 // 256 MB

		r.Body = http.MaxBytesReader(w, r.Body, maxSnapshotSize)

		tempFile, err := os.CreateTemp(dbInstance.Dir(), "snapshot_restore_*.db")
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create temporary file", err, logger.APILog)
			return
		}

		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}()

		if _, err := io.Copy(tempFile, r.Body); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Failed to read snapshot body", err, logger.APILog)
			return
		}

		if _, err := tempFile.Seek(0, 0); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to reset file pointer", err, logger.APILog)
			return
		}

		if err := dbInstance.RestoreRaftSnapshot(r.Context(), tempFile); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to restore snapshot", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Snapshot restored"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			r.Context(),
			SnapshotRestoreAction,
			emailStr,
			getClientIP(r),
			"Restored Raft snapshot",
		)

		logger.APILog.Info("Raft snapshot restored via API", zap.String("user", emailStr))
	})
}
