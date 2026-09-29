// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// Audit log actions for the location endpoint. Location lookups reveal a
// subscriber's physical position and are always audited.
const (
	GetSubscriberLocationAction = "get_subscriber_location"
	CancelLocationSessionAction = "cancel_location_session"
)

// LocationRequest is the unified request body for all location operations.
type LocationRequest struct {
	SUPI        string `json:"supi"`
	RequestType string `json:"request_type"`
	Method      string `json:"method"`
	Mode        string `json:"mode,omitempty"`
	SessionID   string `json:"session_id"`
}

func GetSubscriberLocation(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		// Actor for audit logging (best-effort; the request is authenticated).
		email, _ := r.Context().Value(contextKeyEmail).(string)

		requestType := lmf.RequestType(req.RequestType)
		switch requestType {
		case lmf.RequestImmediate, lmf.RequestPeriodic, lmf.RequestTriggered, lmf.RequestCancel:
		default:
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("unsupported request_type: %s", req.RequestType), nil, logger.APILog)

			return
		}

		if requestType != lmf.RequestCancel {
			if req.SUPI == "" {
				writeError(r.Context(), w, http.StatusBadRequest, "supi is required", nil, logger.APILog)
				return
			}

			if _, err := etsi.NewSUPIFromPrefixed(req.SUPI); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid SUPI format", err, logger.APILog)
				return
			}
		}

		if requestType == lmf.RequestCancel {
			if req.SessionID == "" {
				writeError(r.Context(), w, http.StatusBadRequest, "session_id is required for cancel", nil, logger.APILog)
				return
			}

			if err := lmfInstance.SessionManager().CancelSession(r.Context(), req.SessionID); err != nil {
				writeError(r.Context(), w, http.StatusNotFound, "Session not found", err, logger.APILog)
				return
			}

			logger.LogAuditEvent(
				r.Context(),
				CancelLocationSessionAction,
				email,
				getClientIP(r),
				"User cancelled location session: "+req.SessionID,
			)

			w.WriteHeader(http.StatusNoContent)

			return
		}

		method := lmf.RequestedMethod(req.Method)
		if method == "" {
			method = lmf.DefaultMethodForRequest(requestType)
		}

		switch method {
		case lmf.RequestedCellID, lmf.RequestedECID, lmf.RequestedGNSS:
		default:
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("unsupported method: %s", req.Method), nil, logger.APILog)

			return
		}

		mode := lmfmodels.PositioningMode(req.Mode)
		if err := lmf.ValidateMode(method, mode); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("unsupported mode %q for method %s (supported: %s)", req.Mode, method, supportedModes(method)), nil, logger.APILog)

			return
		}

		// Audit the sensitive location lookup up front, so the access attempt is
		// recorded regardless of the outcome (success, not-found, or failure).
		logger.LogAuditEvent(
			r.Context(),
			GetSubscriberLocationAction,
			email,
			getClientIP(r),
			fmt.Sprintf("User requested location for SUPI %s (method=%s, mode=%s, request_type=%s)", req.SUPI, method, req.Mode, req.RequestType),
		)

		if requestType == lmf.RequestImmediate {
			supi, err := etsi.NewSUPIFromPrefixed(req.SUPI)
			if err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid SUPI", err, logger.APILog)
				return
			}

			result, sessionID, err := lmfInstance.DetermineLocation(r.Context(), supi, method, mode)
			if err != nil {
				logger.LmfLog.Warn("Positioning procedure failed",
					zap.String("session_id", sessionID),
					zap.String("method", string(method)),
					zap.String("mode", string(mode)),
					zap.Error(err),
				)
				writeLocationError(r.Context(), w, err)

				return
			}

			writeResponse(r.Context(), w, toLocationData(result), http.StatusOK, logger.APILog)

			return
		}

		sessionID, err := lmfInstance.SessionManager().CreateSession(r.Context(), lmf.CreateSessionParams{
			SUPI:        req.SUPI,
			RequestType: requestType,
			Method:      method,
		})
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create session", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, map[string]string{"id": sessionID}, http.StatusCreated, logger.APILog)
	})
}

func supportedModes(method lmf.RequestedMethod) string {
	modes := lmf.SupportedModes(method)
	if len(modes) == 0 {
		return "none"
	}

	names := make([]string, len(modes))
	for i, m := range modes {
		names[i] = string(m)
	}

	return strings.Join(names, ", ")
}

// writeLocationError maps LMF errors to HTTP responses. A missing UE and an
// unavailable location estimate are both 404 (client-actionable); anything else
// is a 500.
func writeLocationError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, lmf.ErrNotFound):
		writeError(ctx, w, http.StatusNotFound, "UE not found or not registered", err, logger.APILog)
	case errors.Is(err, lmf.ErrNoLocationEstimate):
		writeError(ctx, w, http.StatusNotFound, "location estimate unavailable: no coordinate for serving cell", err, logger.APILog)
	case errors.Is(err, lpp.ErrUENoLocationEstimate):
		writeError(ctx, w, http.StatusNotFound, "location estimate unavailable: UE provided no location estimate", err, logger.APILog)
	case errors.Is(err, lpp.ErrUnsupportedLocationShape):
		writeError(ctx, w, http.StatusNotFound, "location estimate unavailable: UE location estimate uses an unsupported shape", err, logger.APILog)
	case errors.Is(err, context.DeadlineExceeded):
		writeError(ctx, w, http.StatusGatewayTimeout, "location request timed out", err, logger.APILog)
	default:
		writeError(ctx, w, http.StatusInternalServerError, "Failed to determine location", err, logger.APILog)
	}
}
