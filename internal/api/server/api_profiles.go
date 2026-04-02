package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreateProfileParams struct {
	Name           string `json:"name"`
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type UpdateProfileParams struct {
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type ProfileResponse struct {
	Name           string `json:"name"`
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type ListProfilesResponse struct {
	Items      []ProfileResponse `json:"items"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
	TotalCount int               `json:"total_count"`
}

const (
	CreateProfileAction = "create_profile"
	UpdateProfileAction = "update_profile"
	DeleteProfileAction = "delete_profile"
)

const MaxNumProfiles = 12

func ListProfiles(dbInstance *db.Database) http.Handler {
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

		dbProfiles, total, err := dbInstance.ListProfilesPage(r.Context(), page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list profiles", err, logger.APILog)
			return
		}

		items := make([]ProfileResponse, 0, len(dbProfiles))
		for _, p := range dbProfiles {
			items = append(items, ProfileResponse{
				Name:           p.Name,
				UeAmbrUplink:   p.UeAmbrUplink,
				UeAmbrDownlink: p.UeAmbrDownlink,
			})
		}

		writeResponse(r.Context(), w, ListProfilesResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}, http.StatusOK, logger.APILog)
	})
}

func GetProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		dbProfile, err := dbInstance.GetProfile(r.Context(), name)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, ProfileResponse{
			Name:           dbProfile.Name,
			UeAmbrUplink:   dbProfile.UeAmbrUplink,
			UeAmbrDownlink: dbProfile.UeAmbrDownlink,
		}, http.StatusOK, logger.APILog)
	})
}

func CreateProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateProfileParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
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

		if params.UeAmbrUplink == "" || params.UeAmbrDownlink == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "ue_ambr_uplink and ue_ambr_downlink are required", nil, logger.APILog)
			return
		}

		if !isValidBitrate(params.UeAmbrUplink) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid ue_ambr_uplink format", nil, logger.APILog)
			return
		}

		if !isValidBitrate(params.UeAmbrDownlink) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid ue_ambr_downlink format", nil, logger.APILog)
			return
		}

		numProfiles, err := dbInstance.CountProfiles(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count profiles", err, logger.APILog)
			return
		}

		if numProfiles >= MaxNumProfiles {
			writeError(r.Context(), w, http.StatusConflict, "Maximum number of profiles reached", nil, logger.APILog)
			return
		}

		profile := &db.Profile{
			Name:           params.Name,
			UeAmbrUplink:   params.UeAmbrUplink,
			UeAmbrDownlink: params.UeAmbrDownlink,
		}

		if err := dbInstance.CreateProfile(r.Context(), profile); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "Profile already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create profile", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Profile created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateProfileAction, email, getClientIP(r), "User created profile: "+params.Name)
	})
}

func UpdateProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		var params UpdateProfileParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.UeAmbrUplink == "" || params.UeAmbrDownlink == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "ue_ambr_uplink and ue_ambr_downlink are required", nil, logger.APILog)
			return
		}

		if !isValidBitrate(params.UeAmbrUplink) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid ue_ambr_uplink format", nil, logger.APILog)
			return
		}

		if !isValidBitrate(params.UeAmbrDownlink) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid ue_ambr_downlink format", nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetProfile(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)

			return
		}

		profile := &db.Profile{
			Name:           name,
			UeAmbrUplink:   params.UeAmbrUplink,
			UeAmbrDownlink: params.UeAmbrDownlink,
		}

		if err := dbInstance.UpdateProfile(r.Context(), profile); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update profile", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Profile updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateProfileAction, email, getClientIP(r), "User updated profile: "+name)
	})
}

func DeleteProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetProfile(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)

			return
		}

		subscribersExist, err := dbInstance.SubscribersInProfile(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check subscribers", err, logger.APILog)
			return
		}

		if subscribersExist {
			writeError(r.Context(), w, http.StatusConflict, "Profile has subscribers", nil, logger.APILog)
			return
		}

		profile, err := dbInstance.GetProfile(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)
			return
		}

		policyCount, err := dbInstance.CountPoliciesInProfile(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check policies", err, logger.APILog)
			return
		}

		if policyCount > 0 {
			writeError(r.Context(), w, http.StatusConflict, "Profile has policies", nil, logger.APILog)
			return
		}

		if err := dbInstance.DeleteProfile(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete profile", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Profile deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteProfileAction, email, getClientIP(r), "User deleted profile: "+name)
	})
}
