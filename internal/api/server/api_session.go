// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/logger"
)

// SessionListItem is a session record for list responses.
type SessionListItem struct {
	ID          string `json:"id"`
	SUPI        string `json:"supi"`
	AMFID       string `json:"amf_id"`
	SessionType int    `json:"session_type"`
	Method      string `json:"method"`
	Status      int    `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// SessionDetail is a session record with result.
type SessionDetail struct {
	ID                string          `json:"id"`
	SUPI              string          `json:"supi"`
	AMFID             string          `json:"amf_id"`
	SessionType       int             `json:"session_type"`
	Method            string          `json:"method"`
	Status            int             `json:"status"`
	QoSResponseTimeMs *int            `json:"qos_response_time_ms,omitempty"`
	QOSHAccuracyM     *int            `json:"qos_horizontal_accuracy_m,omitempty"`
	LastResult        json.RawMessage `json:"last_result,omitempty"`
	CreatedAt         int64           `json:"created_at"`
	UpdatedAt         int64           `json:"updated_at"`
}

func ListSessions(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supi := r.URL.Query().Get("supi")
		if supi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "supi query parameter is required", nil, logger.APILog)
			return
		}

		sessions, err := dbInstance.ListPositioningSessions(r.Context(), supi, -1)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list sessions", err, logger.APILog)
			return
		}

		items := make([]SessionListItem, 0, len(sessions))
		for _, s := range sessions {
			createdAt, _ := time.Parse(time.RFC3339, s.CreatedAt)
			updatedAt, _ := time.Parse(time.RFC3339, s.UpdatedAt)
			items = append(items, SessionListItem{
				ID:          s.ID,
				SUPI:        s.SUPI,
				AMFID:       s.AMFID,
				SessionType: s.SessionType,
				Method:      s.Method,
				Status:      s.Status,
				CreatedAt:   createdAt.Unix(),
				UpdatedAt:   updatedAt.Unix(),
			})
		}

		writeResponse(r.Context(), w, items, http.StatusOK, logger.APILog)
	})
}

func GetSession(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		session, err := dbInstance.GetPositioningSession(r.Context(), id)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Session not found", err, logger.APILog)
			return
		}

		var result json.RawMessage
		if session.LastResult != nil {
			result = json.RawMessage(*session.LastResult)
		}

		createdAt, _ := time.Parse(time.RFC3339, session.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, session.UpdatedAt)
		writeResponse(r.Context(), w, SessionDetail{
			ID:                session.ID,
			SUPI:              session.SUPI,
			AMFID:             session.AMFID,
			SessionType:       session.SessionType,
			Method:            session.Method,
			Status:            session.Status,
			QoSResponseTimeMs: session.QoSResponseTimeMs,
			QOSHAccuracyM:     session.QOSHAccuracyM,
			LastResult:        result,
			CreatedAt:         createdAt.Unix(),
			UpdatedAt:         updatedAt.Unix(),
		}, http.StatusOK, logger.APILog)
	})
}

func CancelSession(lmfInst *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := lmfInst.SessionManager().CancelSession(r.Context(), id); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Session not found", err, logger.APILog)
			} else {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to cancel session", err, logger.APILog)
			}

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
