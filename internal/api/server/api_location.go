// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// LocationRequest is the unified request body for all location operations.
type LocationRequest struct {
	SUPI              string `json:"supi"`
	RequestType       string `json:"request_type"`
	Method            string `json:"method"`
	SessionID         string `json:"session_id"`
	QoSResponseTimeMs *int   `json:"qos_response_time_ms,omitempty"`
	QOSHAccuracyM     *int   `json:"qos_horizontal_accuracy_m,omitempty"`
}

func GetSubscriberLocation(amfInstance *amf.AMF, lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		// verbose=true attaches the non-standard supplementaryMeasurements block.
		verbose := r.URL.Query().Get("verbose") == "true"

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

			w.WriteHeader(http.StatusNoContent)

			return
		}

		if req.Method != "" {
			switch lmf.PositioningMethod(req.Method) {
			case lmf.MethodCellID, lmf.MethodECID, lmf.MethodAGNSSAssisted, lmf.MethodAGNSSBased:
			default:
				writeError(r.Context(), w, http.StatusBadRequest,
					fmt.Sprintf("unsupported method: %s", req.Method), nil, logger.APILog)

				return
			}
		}

		// Cell ID is the only method that returns a result directly without
		// needing session tracking (no LPP/NRPPa exchange required).
		if requestType == lmf.RequestImmediate {
			method := lmf.PositioningMethod(req.Method)
			if method == "" {
				method = lmf.DefaultMethodForRequest(lmf.RequestImmediate)
			}

			if method == lmf.MethodCellID {
				supi, err := etsi.NewSUPIFromPrefixed(req.SUPI)
				if err != nil {
					writeError(r.Context(), w, http.StatusBadRequest, "Invalid SUPI", err, logger.APILog)
					return
				}

				result, _, err := lmfInstance.DetermineLocation(r.Context(), supi, method)
				if err != nil {
					writeLocationError(r.Context(), w, err)
					return
				}

				writeResponse(r.Context(), w, toLocationData(result, verbose), http.StatusOK, logger.APILog)

				return
			}
		}

		supi, err := etsi.NewSUPIFromPrefixed(req.SUPI)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SUPI", err, logger.APILog)
			return
		}

		method := lmf.PositioningMethod(req.Method)
		if method == "" {
			method = lmf.DefaultMethodForRequest(lmf.RequestType(req.RequestType))
		}

		// For methods that require LPP/NRPPa exchange (ECID, A-GNSS), create a
		// session and run the positioning procedure synchronously. The handler
		// completes the session after the procedure returns.
		switch method {
		case lmf.MethodECID:
			sessionID, err := lmfInstance.SessionManager().CreateSession(r.Context(), lmf.CreateSessionParams{
				SUPI:              req.SUPI,
				RequestType:       lmf.RequestType(req.RequestType),
				Method:            method,
				QoSResponseTimeMs: req.QoSResponseTimeMs,
				QOSHAccuracyM:     req.QOSHAccuracyM,
			})
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create session", err, logger.APILog)
				return
			}

			// Run the positioning procedure and complete the session with the result.
			result, _, err := lmfInstance.DetermineLocation(r.Context(), supi, method)
			if err != nil {
				logger.LmfLog.Warn("Positioning procedure failed",
					zap.String("session_id", sessionID),
					zap.String("method", string(method)),
					zap.Error(err),
				)
				_ = lmfInstance.SessionManager().FailSession(r.Context(), sessionID)
				writeLocationError(r.Context(), w, err)

				return
			} else if err := lmfInstance.SessionManager().CompleteSession(r.Context(), sessionID, result); err != nil {
				logger.LmfLog.Warn("Failed to complete session",
					zap.String("session_id", sessionID),
					zap.Error(err),
				)
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to complete session", err, logger.APILog)

				return
			}

			writeResponse(r.Context(), w, toLocationData(result, verbose), http.StatusOK, logger.APILog)

			return

		case lmf.MethodAGNSSAssisted, lmf.MethodAGNSSBased:
			// A-GNSS creates its own LPP session via DetermineLocation.
			// The LPP state machine completes the session when done.
			result, sessionID, err := lmfInstance.DetermineLocation(r.Context(), supi, method)
			if err != nil {
				logger.LmfLog.Warn("Positioning procedure failed",
					zap.String("session_id", sessionID),
					zap.String("method", string(method)),
					zap.Error(err),
				)
				writeLocationError(r.Context(), w, err)

				return
			}

			if sessionID != "" {
				// Session was created and completed by the LPP state machine.
				// Ensure the result is stored for the tester to retrieve.
				if err := lmfInstance.SessionManager().CompleteSession(r.Context(), sessionID, result); err != nil {
					logger.LmfLog.Warn("Failed to store A-GNSS result",
						zap.String("session_id", sessionID),
						zap.Error(err),
					)
				}
			}

			writeResponse(r.Context(), w, toLocationData(result, verbose), http.StatusOK, logger.APILog)

			return
		}

		// For deferred requests (periodic/triggered) with non-ECID/A-GNSS methods,
		// just create the session and return the ID.
		sessionID, err := lmfInstance.SessionManager().CreateSession(r.Context(), lmf.CreateSessionParams{
			SUPI:              req.SUPI,
			RequestType:       lmf.RequestType(req.RequestType),
			Method:            method,
			QoSResponseTimeMs: req.QoSResponseTimeMs,
			QOSHAccuracyM:     req.QOSHAccuracyM,
		})
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create session", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, map[string]string{"id": sessionID}, http.StatusCreated, logger.APILog)
	})
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
	default:
		writeError(ctx, w, http.StatusInternalServerError, "Failed to determine location", err, logger.APILog)
	}
}
