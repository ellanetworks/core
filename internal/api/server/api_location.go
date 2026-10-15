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

const GetSubscriberLocationAction = "get_subscriber_location"

type LocationRequest struct {
	SUPI                  string                 `json:"supi"`
	Method                string                 `json:"method,omitempty"`
	Mode                  string                 `json:"mode,omitempty"`
	MeasurementQuantities *MeasurementQuantities `json:"measurement_quantities,omitempty"`
}

type MeasurementQuantities struct {
	NR    []string `json:"nr,omitempty"`
	EUTRA []string `json:"eutra,omitempty"`
}

func (q *MeasurementQuantities) toModel() *lmfmodels.MeasurementQuantities {
	if q == nil {
		return nil
	}

	return &lmfmodels.MeasurementQuantities{
		NR:    toQuantityList(q.NR),
		EUTRA: toQuantityList(q.EUTRA),
	}
}

func fromQuantitiesModel(q *lmfmodels.MeasurementQuantities) *MeasurementQuantities {
	if q == nil {
		return nil
	}

	out := &MeasurementQuantities{}

	for _, v := range q.NR {
		out.NR = append(out.NR, string(v))
	}

	for _, v := range q.EUTRA {
		out.EUTRA = append(out.EUTRA, string(v))
	}

	return out
}

func toQuantityList(values []string) []lmfmodels.MeasurementQuantity {
	if values == nil {
		return nil
	}

	out := make([]lmfmodels.MeasurementQuantity, len(values))
	for i, v := range values {
		out[i] = lmfmodels.MeasurementQuantity(v)
	}

	return out
}

func decodeStrict(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	return dec.Decode(v)
}

func parseSUPI(raw string) (etsi.SUPI, string) {
	if raw == "" {
		return etsi.SUPI{}, "supi is required"
	}

	supi, err := etsi.NewSUPIFromPrefixed(raw)
	if err != nil {
		return etsi.SUPI{}, "Invalid SUPI format"
	}

	return supi, ""
}

func parseLocationRequest(method, mode string, quantities *MeasurementQuantities) (lmf.LocationRequest, string) {
	req := lmf.LocationRequest{
		Method:                lmf.RequestedMethod(method),
		Mode:                  lmfmodels.PositioningMode(mode),
		MeasurementQuantities: quantities.toModel(),
	}

	if req.Method == "" {
		req.Method = lmf.RequestedCellID
	}

	err := req.Validate()

	switch {
	case err == nil:
		return req, ""
	case errors.Is(err, lmf.ErrUnsupportedMethod):
		return req, fmt.Sprintf("unsupported method: %s", method)
	case errors.Is(err, lmf.ErrUnsupportedMode):
		return req, fmt.Sprintf("unsupported mode %q for method %s (supported: %s)", mode, req.Method, supportedModes(req.Method))
	default:
		return req, err.Error()
	}
}

func GetSubscriberLocation(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LocationRequest
		if err := decodeStrict(r, &req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		supi, msg := parseSUPI(req.SUPI)
		if msg != "" {
			writeError(r.Context(), w, http.StatusBadRequest, msg, nil, logger.APILog)
			return
		}

		locationRequest, msg := parseLocationRequest(req.Method, req.Mode, req.MeasurementQuantities)
		if msg != "" {
			writeError(r.Context(), w, http.StatusBadRequest, msg, nil, logger.APILog)
			return
		}

		email, _ := r.Context().Value(contextKeyEmail).(string)

		logger.LogAuditEvent(
			r.Context(),
			GetSubscriberLocationAction,
			email,
			getClientIP(r),
			fmt.Sprintf("User requested location for SUPI %s (method=%s, mode=%s)", req.SUPI, locationRequest.Method, req.Mode),
		)

		result, err := lmfInstance.DetermineLocation(r.Context(), supi, locationRequest)
		if err != nil {
			logger.LmfLog.Warn("Positioning procedure failed",
				zap.String("method", string(locationRequest.Method)),
				zap.String("mode", req.Mode),
				zap.Error(err),
			)
			writeLocationError(r.Context(), w, err)

			return
		}

		writeResponse(r.Context(), w, toLocationData(result), http.StatusOK, logger.APILog)
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
