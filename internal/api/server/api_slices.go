package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreateSliceParams struct {
	Name string `json:"name"`
	Sst  int    `json:"sst"`
	Sd   string `json:"sd,omitempty"`
}

type UpdateSliceParams struct {
	Sst int    `json:"sst"`
	Sd  string `json:"sd,omitempty"`
}

type SliceResponse struct {
	Name string `json:"name"`
	Sst  int    `json:"sst"`
	Sd   string `json:"sd,omitempty"`
}

type ListSlicesResponse struct {
	Items      []SliceResponse `json:"items"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalCount int             `json:"total_count"`
}

const (
	CreateSliceAction = "create_slice"
	UpdateSliceAction = "update_slice"
	DeleteSliceAction = "delete_slice"
	MaxNumSlices      = 8
)

func sliceResponseFromDB(s *db.NetworkSlice) SliceResponse {
	sd := ""
	if s.Sd != nil {
		sd = *s.Sd
	}

	return SliceResponse{
		Name: s.Name,
		Sst:  int(s.Sst),
		Sd:   sd,
	}
}

func ListSlices(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		dbSlices, total, err := dbInstance.ListNetworkSlicesPage(r.Context(), page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list slices", err, logger.APILog)
			return
		}

		items := make([]SliceResponse, 0, len(dbSlices))
		for _, s := range dbSlices {
			items = append(items, sliceResponseFromDB(&s))
		}

		writeResponse(r.Context(), w, ListSlicesResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}, http.StatusOK, logger.APILog)
	})
}

func GetSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		dbSlice, err := dbInstance.GetNetworkSlice(r.Context(), name)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve slice", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, sliceResponseFromDB(dbSlice), http.StatusOK, logger.APILog)
	})
}

func CreateSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateSliceParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		numSlices, err := dbInstance.CountNetworkSlices(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count slices", err, logger.APILog)
			return
		}

		if numSlices >= MaxNumSlices {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of slices reached ("+strconv.Itoa(MaxNumSlices)+")", nil, logger.APILog)
			return
		}

		if params.Name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "name is missing", nil, logger.APILog)
			return
		}

		if !isPolicyNameValid(params.Name) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid name format - must be less than 256 characters", nil, logger.APILog)
			return
		}

		if params.Sst == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "sst is missing", nil, logger.APILog)
			return
		}

		if !isValidSst(params.Sst) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer", nil, logger.APILog)
			return
		}

		if params.Sd != "" {
			if _, err := ParseSDString(params.Sd); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit hex string", nil, logger.APILog)
				return
			}
		}

		var sdPtr *string
		if params.Sd != "" {
			sdPtr = &params.Sd
		}

		slice := &db.NetworkSlice{
			Name: params.Name,
			Sst:  int32(params.Sst),
			Sd:   sdPtr,
		}

		if err := dbInstance.CreateNetworkSlice(r.Context(), slice); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "Slice already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create slice", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Slice created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateSliceAction, email, getClientIP(r), "User created slice: "+params.Name)
	})
}

func UpdateSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		var params UpdateSliceParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Sst == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "sst is missing", nil, logger.APILog)
			return
		}

		if !isValidSst(params.Sst) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer", nil, logger.APILog)
			return
		}

		if params.Sd != "" {
			if _, err := ParseSDString(params.Sd); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit hex string", nil, logger.APILog)
				return
			}
		}

		if _, err := dbInstance.GetNetworkSlice(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve slice", err, logger.APILog)

			return
		}

		var sdPtr *string
		if params.Sd != "" {
			sdPtr = &params.Sd
		}

		slice := &db.NetworkSlice{
			Name: name,
			Sst:  int32(params.Sst),
			Sd:   sdPtr,
		}

		if err := dbInstance.UpdateNetworkSlice(r.Context(), slice); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update slice", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Slice updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateSliceAction, email, getClientIP(r), "User updated slice: "+name)
	})
}

func DeleteSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetNetworkSlice(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve slice", err, logger.APILog)

			return
		}

		sliceCount, err := dbInstance.CountNetworkSlices(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count slices", err, logger.APILog)
			return
		}

		if sliceCount <= 1 {
			writeError(r.Context(), w, http.StatusConflict, "Cannot delete the last network slice", nil, logger.APILog)
			return
		}

		policiesExist, err := dbInstance.PoliciesInSlice(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check policies", err, logger.APILog)
			return
		}

		if policiesExist {
			writeError(r.Context(), w, http.StatusConflict, "Slice has policies", nil, logger.APILog)
			return
		}

		if err := dbInstance.DeleteNetworkSlice(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete slice", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Slice deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSliceAction, email, getClientIP(r), "User deleted slice: "+name)
	})
}
