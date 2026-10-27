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
	"github.com/ellanetworks/core/internal/hss/psi"
	"github.com/ellanetworks/core/internal/logger"
)

type IMSProfileNotifier interface {
	ApplicationServerChanged(ctx context.Context, applicationServerID string)
	ProfileChanged(ctx context.Context, profileID string)
	ServiceProfileChanged(ctx context.Context, serviceProfileID string)
	SubscriberChanged(ctx context.Context, imsi string)
	AllSubscribersChanged(ctx context.Context)
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
	Priority                int            `json:"priority"`
	ApplicationServerName   string         `json:"application_server_name"`
	Trigger                 *db.IMSTrigger `json:"trigger,omitempty"`
	ProfilePart             *string        `json:"profile_part,omitempty"`
	DefaultHandling         *string        `json:"default_handling,omitempty"`
	ServiceInfo             *string        `json:"service_info,omitempty"`
	IncludeRegisterRequest  bool           `json:"include_register_request"`
	IncludeRegisterResponse bool           `json:"include_register_response"`
}

type IMSServiceProfile struct {
	Name                  string                      `json:"name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ListIMSServiceProfilesResponse struct {
	Items []IMSServiceProfile `json:"items"`
}

type CreateIMSServiceProfileParams struct {
	Name                  string                      `json:"name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type UpdateIMSServiceProfileParams struct {
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ProfileIMSServiceProfiles struct {
	MMTel *string `json:"mmtel"`
	MCPTT *string `json:"mcptt"`
}

const (
	CreateIMSApplicationServerAction      = "create_ims_application_server"
	UpdateIMSApplicationServerAction      = "update_ims_application_server"
	DeleteIMSApplicationServerAction      = "delete_ims_application_server"
	CreateIMSServiceProfileAction         = "create_ims_service_profile"
	UpdateIMSServiceProfileAction         = "update_ims_service_profile"
	DeleteIMSServiceProfileAction         = "delete_ims_service_profile"
	UpdateProfileIMSServiceProfilesAction = "update_profile_ims_service_profiles"
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
			Trigger:                 c.Trigger,
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
			Trigger:                 c.Trigger,
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
		writeError(r.Context(), w, http.StatusConflict, "Application server is used by a service profile", nil, logger.APILog)
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

		if serverName, _ := psi.CanonicalServerName(as.ServerName); notifier != nil && serverName != existing.ServerName {
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

func imsApplicationServerNames(r *http.Request, dbInstance *db.Database) (map[string]string, error) {
	servers, err := dbInstance.ListIMSApplicationServers(r.Context())
	if err != nil {
		return nil, err
	}

	names := make(map[string]string, len(servers))
	for _, as := range servers {
		names[as.ID] = as.Name
	}

	return names, nil
}

func imsServiceProfileResponse(sp db.IMSServiceProfile, applicationServerNames map[string]string) IMSServiceProfile {
	return IMSServiceProfile{Name: sp.Name, InitialFilterCriteria: imsFilterCriteriaResponse(sp.InitialFilterCriteria, applicationServerNames)}
}

func writeIMSServiceProfileError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, "Service profile or application server not found", nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, "Service profile already exists", nil, logger.APILog)
	case errors.Is(err, db.ErrInUse):
		writeError(r.Context(), w, http.StatusConflict, "Service profile is assigned to a profile", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" service profile", err, logger.APILog)
	}
}

func imsFilterCriteriaOfParams(w http.ResponseWriter, r *http.Request, dbInstance *db.Database, params []IMSInitialFilterCriterion) ([]db.IMSInitialFilterCriterion, bool) {
	ids := make(map[string]string)

	for _, c := range params {
		if _, known := ids[c.ApplicationServerName]; known {
			continue
		}

		if c.ApplicationServerName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Each initial filter criterion needs an application server", nil, logger.APILog)
			return nil, false
		}

		as, err := dbInstance.GetIMSApplicationServer(r.Context(), c.ApplicationServerName)
		if errors.Is(err, db.ErrNotFound) {
			writeError(r.Context(), w, http.StatusBadRequest, "Application server not found: "+c.ApplicationServerName, nil, logger.APILog)
			return nil, false
		}

		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get application server", err, logger.APILog)
			return nil, false
		}

		ids[c.ApplicationServerName] = as.ID
	}

	ifcs := imsFilterCriteriaFromParams(params, ids)

	if err := db.ValidateIMSFilterCriteria(ifcs); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "Invalid initial filter criteria: "+err.Error(), nil, logger.APILog)
		return nil, false
	}

	return ifcs, true
}

func ListIMSServiceProfiles(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profiles, err := dbInstance.ListIMSServiceProfiles(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list service profiles", err, logger.APILog)
			return
		}

		names, err := imsApplicationServerNames(r, dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list application servers", err, logger.APILog)
			return
		}

		resp := ListIMSServiceProfilesResponse{Items: make([]IMSServiceProfile, 0, len(profiles))}
		for _, sp := range profiles {
			resp.Items = append(resp.Items, imsServiceProfileResponse(sp, names))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetIMSServiceProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sp, err := dbInstance.GetIMSServiceProfile(r.Context(), r.PathValue("name"))
		if err != nil {
			writeIMSServiceProfileError(w, r, "get", err)
			return
		}

		names, err := imsApplicationServerNames(r, dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list application servers", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, imsServiceProfileResponse(*sp, names), http.StatusOK, logger.APILog)
	})
}

func CreateIMSServiceProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateIMSServiceProfileParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Name == "" || !isResourceNameValid(params.Name) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid name format - must be less than 256 characters", nil, logger.APILog)
			return
		}

		ifcs, ok := imsFilterCriteriaOfParams(w, r, dbInstance, params.InitialFilterCriteria)
		if !ok {
			return
		}

		sp := db.IMSServiceProfile{Name: params.Name, InitialFilterCriteria: ifcs}

		if err := dbInstance.CreateIMSServiceProfile(r.Context(), &sp); err != nil {
			writeIMSServiceProfileError(w, r, "create", err)
			return
		}

		names, err := imsApplicationServerNames(r, dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list application servers", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, imsServiceProfileResponse(sp, names), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateIMSServiceProfileAction, email, getClientIP(r),
			fmt.Sprintf("Created service profile %s (%d initial filter criteria)", sp.Name, len(ifcs)))
	})
}

func UpdateIMSServiceProfile(dbInstance *db.Database, notifier IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params UpdateIMSServiceProfileParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		existing, err := dbInstance.GetIMSServiceProfile(r.Context(), r.PathValue("name"))
		if err != nil {
			writeIMSServiceProfileError(w, r, "update", err)
			return
		}

		ifcs, ok := imsFilterCriteriaOfParams(w, r, dbInstance, params.InitialFilterCriteria)
		if !ok {
			return
		}

		if err := dbInstance.UpdateIMSServiceProfile(r.Context(), &db.IMSServiceProfile{ID: existing.ID, Name: existing.Name, InitialFilterCriteria: ifcs}); err != nil {
			writeIMSServiceProfileError(w, r, "update", err)
			return
		}

		if notifier != nil {
			notifier.ServiceProfileChanged(context.WithoutCancel(r.Context()), existing.ID)
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Service profile updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateIMSServiceProfileAction, email, getClientIP(r),
			fmt.Sprintf("Updated service profile %s (%d initial filter criteria)", existing.Name, len(ifcs)))
	})
}

func DeleteIMSServiceProfile(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		name := r.PathValue("name")

		if err := dbInstance.DeleteIMSServiceProfile(r.Context(), name); err != nil {
			writeIMSServiceProfileError(w, r, "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Service profile deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteIMSServiceProfileAction, email, getClientIP(r), "Deleted service profile "+name)
	})
}

func GetProfileIMSServiceProfiles(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profile, ok := lookupProfile(w, r, dbInstance)
		if !ok {
			return
		}

		assigned, err := dbInstance.GetProfileIMSServiceProfiles(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the profile's service profiles", err, logger.APILog)
			return
		}

		serviceProfiles, err := dbInstance.ListIMSServiceProfiles(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list service profiles", err, logger.APILog)
			return
		}

		names := make(map[string]string, len(serviceProfiles))
		for _, sp := range serviceProfiles {
			names[sp.ID] = sp.Name
		}

		var resp ProfileIMSServiceProfiles

		if id, ok := assigned[db.IMSRegistrationSetMMTel]; ok {
			resp.MMTel = new(names[id])
		}

		if id, ok := assigned[db.IMSRegistrationSetMCPTT]; ok {
			resp.MCPTT = new(names[id])
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func UpdateProfileIMSServiceProfiles(dbInstance *db.Database, notifier IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params ProfileIMSServiceProfiles
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		profile, ok := lookupProfile(w, r, dbInstance)
		if !ok {
			return
		}

		assigned := make(map[db.IMSRegistrationSet]string)
		summary := make([]string, 0, 2)

		for _, a := range []struct {
			set  db.IMSRegistrationSet
			name *string
		}{{db.IMSRegistrationSetMMTel, params.MMTel}, {db.IMSRegistrationSetMCPTT, params.MCPTT}} {
			if a.name == nil || *a.name == "" {
				summary = append(summary, string(a.set)+": none")
				continue
			}

			sp, err := dbInstance.GetIMSServiceProfile(r.Context(), *a.name)
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusBadRequest, "Service profile not found: "+*a.name, nil, logger.APILog)
				return
			}

			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get service profile", err, logger.APILog)
				return
			}

			assigned[a.set] = sp.ID
			summary = append(summary, string(a.set)+": "+sp.Name)
		}

		if err := dbInstance.SetProfileIMSServiceProfiles(r.Context(), profile.ID, assigned); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Profile or service profile not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update the profile's service profiles", err, logger.APILog)

			return
		}

		if notifier != nil {
			notifier.ProfileChanged(context.WithoutCancel(r.Context()), profile.ID)
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Profile service profiles updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateProfileIMSServiceProfilesAction, email, getClientIP(r),
			fmt.Sprintf("Updated the IMS service profiles of profile %s (%s)", profile.Name, strings.Join(summary, ", ")))
	})
}
