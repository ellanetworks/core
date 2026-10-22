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

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type IMSProfileNotifier interface {
	ApplicationServerChanged(ctx context.Context, applicationServerID string)
	ProfileChanged(ctx context.Context, profileID string)
	SubscriberChanged(ctx context.Context, imsi string)
	SubscriberIdentitiesChanged(ctx context.Context, imsi, previousMSISDN string)
}

type IMSApplicationServer struct {
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type ListIMSApplicationServersResponse struct {
	Items []IMSApplicationServer `json:"items"`
}

type CreateIMSApplicationServerParams struct {
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type UpdateIMSApplicationServerParams struct {
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type IMSInitialFilterCriterion struct {
	Priority                int                 `json:"priority"`
	ApplicationServerName   string              `json:"application_server_name"`
	TriggerPoint            *db.IMSTriggerPoint `json:"trigger_point,omitempty"`
	ProfilePart             *string             `json:"profile_part,omitempty"`
	DefaultHandling         *string             `json:"default_handling,omitempty"`
	ServiceInfo             *string             `json:"service_info,omitempty"`
	IncludeRegisterRequest  bool                `json:"include_register_request"`
	IncludeRegisterResponse bool                `json:"include_register_response"`
}

type ProfileIMSFilterCriteria struct {
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

const (
	CreateIMSApplicationServerAction     = "create_ims_application_server"
	UpdateIMSApplicationServerAction     = "update_ims_application_server"
	DeleteIMSApplicationServerAction     = "delete_ims_application_server"
	UpdateProfileIMSFilterCriteriaAction = "update_profile_ims_filter_criteria"
)

func imsApplicationServerResponse(as db.IMSApplicationServer) IMSApplicationServer {
	identities := as.PublicServiceIdentities
	if identities == nil {
		identities = []string{}
	}

	return IMSApplicationServer{Name: as.Name, ServerName: as.ServerName, PublicServiceIdentities: identities}
}

func imsApplicationServerSummary(as db.IMSApplicationServer) string {
	return fmt.Sprintf("server name %s, public service identities [%s]", as.ServerName, strings.Join(as.PublicServiceIdentities, " "))
}

func imsFilterCriteriaResponse(criteria []db.IMSInitialFilterCriterion, applicationServerNames map[string]string) []IMSInitialFilterCriterion {
	out := make([]IMSInitialFilterCriterion, 0, len(criteria))

	for i, c := range criteria {
		ifc := IMSInitialFilterCriterion{
			Priority:                i,
			ApplicationServerName:   applicationServerNames[c.ApplicationServerID],
			TriggerPoint:            c.TriggerPoint,
			ServiceInfo:             c.ServiceInfo,
			IncludeRegisterRequest:  c.IncludeRegisterRequest,
			IncludeRegisterResponse: c.IncludeRegisterResponse,
		}

		if c.ProfilePart != nil {
			ifc.ProfilePart = new(string(*c.ProfilePart))
		}

		if c.DefaultHandling != nil {
			ifc.DefaultHandling = new(string(*c.DefaultHandling))
		}

		out = append(out, ifc)
	}

	return out
}

func imsFilterCriteriaFromParams(criteria []IMSInitialFilterCriterion, applicationServerIDs map[string]string) []db.IMSInitialFilterCriterion {
	out := make([]db.IMSInitialFilterCriterion, 0, len(criteria))

	for _, c := range criteria {
		ifc := db.IMSInitialFilterCriterion{
			ApplicationServerID:     applicationServerIDs[c.ApplicationServerName],
			TriggerPoint:            c.TriggerPoint,
			ServiceInfo:             c.ServiceInfo,
			IncludeRegisterRequest:  c.IncludeRegisterRequest,
			IncludeRegisterResponse: c.IncludeRegisterResponse,
		}

		if c.ProfilePart != nil {
			ifc.ProfilePart = new(db.IMSProfilePart(*c.ProfilePart))
		}

		if c.DefaultHandling != nil {
			ifc.DefaultHandling = new(db.IMSDefaultHandling(*c.DefaultHandling))
		}

		out = append(out, ifc)
	}

	return out
}

func psiConflictReason(err error) string {
	msg := err.Error()

	if i := strings.Index(msg, db.ErrPSIConflict.Error()); i >= 0 {
		return msg[i:]
	}

	return msg
}

func writeIMSApplicationServerError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, "Application server not found", nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, "Application server already exists", nil, logger.APILog)
	case errors.Is(err, db.ErrPSIConflict):
		writeError(r.Context(), w, http.StatusConflict, psiConflictReason(err), nil, logger.APILog)
	case errors.Is(err, db.ErrInUse):
		writeError(r.Context(), w, http.StatusConflict, "Application server is used by a profile", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" application server", err, logger.APILog)
	}
}

func ListIMSApplicationServers(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servers, err := dbInstance.ListIMSApplicationServers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list application servers", err, logger.APILog)
			return
		}

		resp := ListIMSApplicationServersResponse{Items: make([]IMSApplicationServer, 0, len(servers))}

		for _, as := range servers {
			resp.Items = append(resp.Items, imsApplicationServerResponse(as))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetIMSApplicationServer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		as, err := dbInstance.GetIMSApplicationServer(r.Context(), r.PathValue("name"))
		if err != nil {
			writeIMSApplicationServerError(w, r, "get", err)
			return
		}

		writeResponse(r.Context(), w, imsApplicationServerResponse(*as), http.StatusOK, logger.APILog)
	})
}

func CreateIMSApplicationServer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateIMSApplicationServerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Name != "" && !isResourceNameValid(params.Name) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid name format - must be less than 256 characters", nil, logger.APILog)
			return
		}

		as := db.IMSApplicationServer{Name: params.Name, ServerName: params.ServerName, PublicServiceIdentities: params.PublicServiceIdentities}

		if err := as.Validate(); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := dbInstance.CreateIMSApplicationServer(r.Context(), &as); err != nil {
			writeIMSApplicationServerError(w, r, "create", err)
			return
		}

		writeResponse(r.Context(), w, imsApplicationServerResponse(as), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateIMSApplicationServerAction, email, getClientIP(r), "Created application server "+as.Name+" ("+imsApplicationServerSummary(as)+")")
	})
}

func UpdateIMSApplicationServer(dbInstance *db.Database, notifier IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params UpdateIMSApplicationServerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		existing, err := dbInstance.GetIMSApplicationServer(r.Context(), r.PathValue("name"))
		if err != nil {
			writeIMSApplicationServerError(w, r, "update", err)
			return
		}

		as := db.IMSApplicationServer{ID: existing.ID, Name: existing.Name, ServerName: params.ServerName, PublicServiceIdentities: params.PublicServiceIdentities}

		if err := as.Validate(); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateIMSApplicationServer(r.Context(), &as); err != nil {
			writeIMSApplicationServerError(w, r, "update", err)
			return
		}

		if notifier != nil {
			notifier.ApplicationServerChanged(context.WithoutCancel(r.Context()), existing.ID)
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Application server updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateIMSApplicationServerAction, email, getClientIP(r), "Updated application server "+as.Name+" ("+imsApplicationServerSummary(as)+")")
	})
}

func DeleteIMSApplicationServer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		name := r.PathValue("name")

		as, err := dbInstance.GetIMSApplicationServer(r.Context(), name)
		if err != nil {
			writeIMSApplicationServerError(w, r, "delete", err)
			return
		}

		if err := dbInstance.DeleteIMSApplicationServer(r.Context(), name); err != nil {
			writeIMSApplicationServerError(w, r, "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Application server deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteIMSApplicationServerAction, email, getClientIP(r), "Deleted application server "+name+" ("+imsApplicationServerSummary(*as)+")")
	})
}

func lookupProfile(w http.ResponseWriter, r *http.Request, dbInstance *db.Database) (*db.Profile, bool) {
	profile, err := dbInstance.GetProfile(r.Context(), r.PathValue("name"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
		return nil, false
	}

	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)
		return nil, false
	}

	return profile, true
}

func GetProfileIMSFilterCriteria(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profile, ok := lookupProfile(w, r, dbInstance)
		if !ok {
			return
		}

		ifcs, err := dbInstance.ListProfileIMSFilterCriteria(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list the profile's initial filter criteria", err, logger.APILog)
			return
		}

		servers, err := dbInstance.ListIMSApplicationServers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list application servers", err, logger.APILog)
			return
		}

		names := make(map[string]string, len(servers))
		for _, as := range servers {
			names[as.ID] = as.Name
		}

		writeResponse(r.Context(), w, ProfileIMSFilterCriteria{InitialFilterCriteria: imsFilterCriteriaResponse(ifcs, names)}, http.StatusOK, logger.APILog)
	})
}

func UpdateProfileIMSFilterCriteria(dbInstance *db.Database, notifier IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params ProfileIMSFilterCriteria
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		profile, ok := lookupProfile(w, r, dbInstance)
		if !ok {
			return
		}

		ids := make(map[string]string)

		for _, c := range params.InitialFilterCriteria {
			if _, known := ids[c.ApplicationServerName]; known {
				continue
			}

			if c.ApplicationServerName == "" {
				writeError(r.Context(), w, http.StatusBadRequest, "Each initial filter criterion needs an application server", nil, logger.APILog)
				return
			}

			as, err := dbInstance.GetIMSApplicationServer(r.Context(), c.ApplicationServerName)
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusBadRequest, "Application server not found: "+c.ApplicationServerName, nil, logger.APILog)
				return
			}

			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get application server", err, logger.APILog)
				return
			}

			ids[c.ApplicationServerName] = as.ID
		}

		ifcs := imsFilterCriteriaFromParams(params.InitialFilterCriteria, ids)

		if err := db.ValidateIMSFilterCriteria(ifcs); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid initial filter criteria: "+err.Error(), nil, logger.APILog)
			return
		}

		if err := dbInstance.SetProfileIMSFilterCriteria(r.Context(), profile.ID, ifcs); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Profile or application server not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update the profile's initial filter criteria", err, logger.APILog)

			return
		}

		if notifier != nil {
			notifier.ProfileChanged(context.WithoutCancel(r.Context()), profile.ID)
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Profile initial filter criteria updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateProfileIMSFilterCriteriaAction, email, getClientIP(r),
			fmt.Sprintf("Updated the initial filter criteria of profile %s (%d)", profile.Name, len(params.InitialFilterCriteria)))
	})
}
