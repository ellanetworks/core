// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

// Audit log actions for cell-position provisioning (mutations).
const (
	CreateCellPositionAction = "create_cell_position"
	UpdateCellPositionAction = "update_cell_position"
	DeleteCellPositionAction = "delete_cell_position"
)

// CellPositionRequest is the provisioning payload for a cell antenna position.
type CellPositionRequest struct {
	RAT                  string   `json:"rat"` // "nr" or "eutra"
	Mcc                  string   `json:"mcc"`
	Mnc                  string   `json:"mnc"`
	CellIdentity         string   `json:"cell_identity"` // hex NCI (NR) or ECI (E-UTRA)
	GNbID                *string  `json:"gnb_id,omitempty"`
	Latitude             float64  `json:"latitude"`  // WGS-84 decimal degrees
	Longitude            float64  `json:"longitude"` // WGS-84 decimal degrees
	Altitude             *float64 `json:"altitude,omitempty"`
	UncertaintySemiMajor *float64 `json:"uncertainty_semi_major,omitempty"` // metres
	UncertaintySemiMinor *float64 `json:"uncertainty_semi_minor,omitempty"` // metres
	OrientationMajor     *int     `json:"orientation_major,omitempty"`      // degrees
	Confidence           *int     `json:"confidence,omitempty"`             // percent
}

// CellPositionResponse is a provisioned cell position record.
type CellPositionResponse struct {
	ID                   string   `json:"id"`
	RAT                  string   `json:"rat"`
	Mcc                  string   `json:"mcc"`
	Mnc                  string   `json:"mnc"`
	CellIdentity         string   `json:"cell_identity"`
	GNbID                *string  `json:"gnb_id,omitempty"`
	Latitude             float64  `json:"latitude"`
	Longitude            float64  `json:"longitude"`
	Altitude             *float64 `json:"altitude,omitempty"`
	UncertaintySemiMajor *float64 `json:"uncertainty_semi_major,omitempty"`
	UncertaintySemiMinor *float64 `json:"uncertainty_semi_minor,omitempty"`
	OrientationMajor     *int     `json:"orientation_major,omitempty"`
	Confidence           *int     `json:"confidence,omitempty"`
	Source               string   `json:"source"`
}

func cellPositionToResponse(c *db.CellPosition) CellPositionResponse {
	return CellPositionResponse{
		ID:                   c.ID,
		RAT:                  c.RAT,
		Mcc:                  c.Mcc,
		Mnc:                  c.Mnc,
		CellIdentity:         c.CellIdentity,
		GNbID:                c.GNbID,
		Latitude:             c.Latitude,
		Longitude:            c.Longitude,
		Altitude:             c.Altitude,
		UncertaintySemiMajor: c.UncertaintySemiMajor,
		UncertaintySemiMinor: c.UncertaintySemiMinor,
		OrientationMajor:     c.OrientationMajor,
		Confidence:           c.Confidence,
		Source:               c.Source,
	}
}

// validate checks a provisioning payload.
func (req *CellPositionRequest) validate() error {
	switch req.RAT {
	case db.RATNR, db.RATEUTRA:
	default:
		return fmt.Errorf("rat must be %q or %q", db.RATNR, db.RATEUTRA)
	}

	if req.Mcc == "" || req.Mnc == "" {
		return errors.New("mcc and mnc are required")
	}

	if req.CellIdentity == "" {
		return errors.New("cell_identity is required")
	}

	if req.Latitude < -90 || req.Latitude > 90 {
		return errors.New("latitude must be within [-90, 90]")
	}

	if req.Longitude < -180 || req.Longitude > 180 {
		return errors.New("longitude must be within [-180, 180]")
	}

	return nil
}

func (req *CellPositionRequest) toModel() *db.CellPosition {
	return &db.CellPosition{
		RAT:                  req.RAT,
		Mcc:                  req.Mcc,
		Mnc:                  req.Mnc,
		CellIdentity:         req.CellIdentity,
		GNbID:                req.GNbID,
		Latitude:             req.Latitude,
		Longitude:            req.Longitude,
		Altitude:             req.Altitude,
		UncertaintySemiMajor: req.UncertaintySemiMajor,
		UncertaintySemiMinor: req.UncertaintySemiMinor,
		OrientationMajor:     req.OrientationMajor,
		Confidence:           req.Confidence,
	}
}

func ListCellPositions(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		positions, err := dbInstance.ListCellPositions(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list cell positions", err, logger.APILog)
			return
		}

		items := make([]CellPositionResponse, 0, len(positions))
		for i := range positions {
			items = append(items, cellPositionToResponse(&positions[i]))
		}

		writeResponse(r.Context(), w, items, http.StatusOK, logger.APILog)
	})
}

func GetCellPosition(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := dbInstance.GetCellPosition(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Cell position not found", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, cellPositionToResponse(c), http.StatusOK, logger.APILog)
	})
}

func CreateCellPosition(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var req CellPositionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if err := req.validate(); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), err, logger.APILog)
			return
		}

		model := req.toModel()
		if err := dbInstance.CreateCellPosition(r.Context(), model); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create cell position", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, CreateSuccessResponse{Message: "Cell position created", ID: model.ID}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateCellPositionAction, email, getClientIP(r), fmt.Sprintf("User created cell position %s for %s", cellDescriptor(&req), model.ID))
	})
}

func UpdateCellPosition(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var req CellPositionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if err := req.validate(); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), err, logger.APILog)
			return
		}

		model := req.toModel()
		model.ID = r.PathValue("id")

		if err := dbInstance.UpdateCellPosition(r.Context(), model); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cell position not found", err, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update cell position", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cell position updated"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateCellPositionAction, email, getClientIP(r), fmt.Sprintf("User updated cell position %s (%s)", model.ID, cellDescriptor(&req)))
	})
}

func DeleteCellPosition(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		id := r.PathValue("id")

		if err := dbInstance.DeleteCellPosition(r.Context(), id); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cell position not found", err, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete cell position", err, logger.APILog)

			return
		}

		w.WriteHeader(http.StatusNoContent)

		logger.LogAuditEvent(r.Context(), DeleteCellPositionAction, email, getClientIP(r), "User deleted cell position: "+id)
	})
}

// cellDescriptor renders a short human-readable identity of a provisioned cell
// for audit-log details.
func cellDescriptor(req *CellPositionRequest) string {
	return fmt.Sprintf("%s %s/%s %s", req.RAT, req.Mcc, req.Mnc, req.CellIdentity)
}
