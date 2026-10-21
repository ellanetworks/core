// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type IMSProfileNotifier interface {
	ApplicationServerChanged(ctx context.Context, applicationServerID string)
	ServiceChanged(ctx context.Context, serviceID string)
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
	Priority                int                `json:"priority"`
	TriggerPoint            db.IMSTriggerPoint `json:"trigger_point"`
	DefaultHandling         string             `json:"default_handling"`
	IncludeRegisterRequest  bool               `json:"include_register_request"`
	IncludeRegisterResponse bool               `json:"include_register_response"`
}

type IMSService struct {
	Name                  string                      `json:"name"`
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ListIMSServicesResponse struct {
	Items []IMSService `json:"items"`
}

type CreateIMSServiceParams struct {
	Name                  string                      `json:"name"`
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type UpdateIMSServiceParams struct {
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ProfileIMSServices struct {
	Services []string `json:"services"`
}

const (
	CreateIMSApplicationServerAction = "create_ims_application_server"
	UpdateIMSApplicationServerAction = "update_ims_application_server"
	DeleteIMSApplicationServerAction = "delete_ims_application_server"
	CreateIMSServiceAction           = "create_ims_service"
	UpdateIMSServiceAction           = "update_ims_service"
	DeleteIMSServiceAction           = "delete_ims_service"
	UpdateProfileIMSServicesAction   = "update_profile_ims_services"
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

func imsServiceResponse(svc db.IMSService, applicationServerName string) IMSService {
	criteria := svc.FilterCriteria()
	out := IMSService{
		Name:                  svc.Name,
		Kind:                  string(svc.Kind),
		ApplicationServerName: applicationServerName,
		InitialFilterCriteria: make([]IMSInitialFilterCriterion, 0, len(criteria)),
	}

	for i, c := range criteria {
		out.InitialFilterCriteria = append(out.InitialFilterCriteria, IMSInitialFilterCriterion{
			Priority:                i,
			TriggerPoint:            c.TriggerPoint,
			DefaultHandling:         string(c.DefaultHandling),
			IncludeRegisterRequest:  c.IncludeRegisterRequest,
			IncludeRegisterResponse: c.IncludeRegisterResponse,
		})
	}

	return out
}

func imsFilterCriteriaFromParams(criteria []IMSInitialFilterCriterion) []db.IMSInitialFilterCriterion {
	if len(criteria) == 0 {
		return nil
	}

	out := make([]db.IMSInitialFilterCriterion, 0, len(criteria))

	for _, c := range criteria {
		out = append(out, db.IMSInitialFilterCriterion{
			TriggerPoint:            c.TriggerPoint,
			DefaultHandling:         db.IMSDefaultHandling(c.DefaultHandling),
			IncludeRegisterRequest:  c.IncludeRegisterRequest,
			IncludeRegisterResponse: c.IncludeRegisterResponse,
		})
	}

	return out
}

func imsServiceSummary(svc db.IMSService, applicationServerName string) string {
	return fmt.Sprintf("kind %s, application server %s, %d initial filter criteria", svc.Kind, applicationServerName, len(svc.FilterCriteria()))
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
		writeError(r.Context(), w, http.StatusConflict, "Application server is used by an IMS service", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" application server", err, logger.APILog)
	}
}

func writeIMSServiceError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, "IMS service not found", nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, "IMS service already exists", nil, logger.APILog)
	case errors.Is(err, db.ErrInUse):
		writeError(r.Context(), w, http.StatusConflict, "IMS service is assigned to a profile", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" IMS service", err, logger.APILog)
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

func ListIMSServices(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		services, err := dbInstance.ListIMSServices(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list IMS services", err, logger.APILog)
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

		resp := ListIMSServicesResponse{Items: make([]IMSService, 0, len(services))}

		for _, svc := range services {
			resp.Items = append(resp.Items, imsServiceResponse(svc, names[svc.ApplicationServerID]))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetIMSService(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc, err := dbInstance.GetIMSService(r.Context(), r.PathValue("name"))
		if err != nil {
			writeIMSServiceError(w, r, "get", err)
			return
		}

		as, err := dbInstance.GetIMSApplicationServerByID(r.Context(), svc.ApplicationServerID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the IMS service's application server", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, imsServiceResponse(*svc, as.Name), http.StatusOK, logger.APILog)
	})
}

func resolveIMSService(w http.ResponseWriter, r *http.Request, dbInstance *db.Database, svc *db.IMSService) (string, bool) {
	applicationServerName := svc.ApplicationServerID

	if err := svc.Validate(); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
		return "", false
	}

	as, err := dbInstance.GetIMSApplicationServer(r.Context(), applicationServerName)
	if errors.Is(err, db.ErrNotFound) {
		writeError(r.Context(), w, http.StatusBadRequest, "Application server not found", nil, logger.APILog)
		return "", false
	}

	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get application server", err, logger.APILog)
		return "", false
	}

	svc.ApplicationServerID = as.ID

	return as.Name, true
}

func CreateIMSService(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateIMSServiceParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Name != "" && !isResourceNameValid(params.Name) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid name format - must be less than 256 characters", nil, logger.APILog)
			return
		}

		svc := db.IMSService{
			Name:                  params.Name,
			Kind:                  db.IMSServiceKind(params.Kind),
			ApplicationServerID:   params.ApplicationServerName,
			InitialFilterCriteria: imsFilterCriteriaFromParams(params.InitialFilterCriteria),
		}

		applicationServerName, ok := resolveIMSService(w, r, dbInstance, &svc)
		if !ok {
			return
		}

		if err := dbInstance.CreateIMSService(r.Context(), &svc); err != nil {
			writeIMSServiceError(w, r, "create", err)
			return
		}

		writeResponse(r.Context(), w, imsServiceResponse(svc, applicationServerName), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateIMSServiceAction, email, getClientIP(r), "Created IMS service "+svc.Name+" ("+imsServiceSummary(svc, applicationServerName)+")")
	})
}

func UpdateIMSService(dbInstance *db.Database, notifier IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params UpdateIMSServiceParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		existing, err := dbInstance.GetIMSService(r.Context(), r.PathValue("name"))
		if err != nil {
			writeIMSServiceError(w, r, "update", err)
			return
		}

		svc := db.IMSService{
			ID:                    existing.ID,
			Name:                  existing.Name,
			Kind:                  db.IMSServiceKind(params.Kind),
			ApplicationServerID:   params.ApplicationServerName,
			InitialFilterCriteria: imsFilterCriteriaFromParams(params.InitialFilterCriteria),
		}

		applicationServerName, ok := resolveIMSService(w, r, dbInstance, &svc)
		if !ok {
			return
		}

		if err := dbInstance.UpdateIMSService(r.Context(), &svc); err != nil {
			writeIMSServiceError(w, r, "update", err)
			return
		}

		if notifier != nil {
			notifier.ServiceChanged(context.WithoutCancel(r.Context()), existing.ID)
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "IMS service updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateIMSServiceAction, email, getClientIP(r), "Updated IMS service "+svc.Name+" ("+imsServiceSummary(svc, applicationServerName)+")")
	})
}

func DeleteIMSService(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		name := r.PathValue("name")

		if err := dbInstance.DeleteIMSService(r.Context(), name); err != nil {
			writeIMSServiceError(w, r, "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "IMS service deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteIMSServiceAction, email, getClientIP(r), "Deleted IMS service "+name)
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

func GetProfileIMSServices(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profile, ok := lookupProfile(w, r, dbInstance)
		if !ok {
			return
		}

		services, err := dbInstance.ListProfileIMSServices(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list the profile's IMS services", err, logger.APILog)
			return
		}

		resp := ProfileIMSServices{Services: make([]string, 0, len(services))}
		for _, svc := range services {
			resp.Services = append(resp.Services, svc.Name)
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func UpdateProfileIMSServices(dbInstance *db.Database, notifier IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params ProfileIMSServices
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		profile, ok := lookupProfile(w, r, dbInstance)
		if !ok {
			return
		}

		ids := make([]string, 0, len(params.Services))

		for i, name := range params.Services {
			if slices.Contains(params.Services[:i], name) {
				writeError(r.Context(), w, http.StatusBadRequest, "IMS service "+name+" is listed twice", nil, logger.APILog)
				return
			}

			svc, err := dbInstance.GetIMSService(r.Context(), name)
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusBadRequest, "IMS service not found: "+name, nil, logger.APILog)
				return
			}

			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get IMS service", err, logger.APILog)
				return
			}

			ids = append(ids, svc.ID)
		}

		if err := dbInstance.SetProfileIMSServices(r.Context(), profile.ID, ids); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				if _, profileErr := dbInstance.GetProfile(r.Context(), profile.Name); profileErr == nil {
					writeError(r.Context(), w, http.StatusBadRequest, "IMS service not found", nil, logger.APILog)
					return
				}

				writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update the profile's IMS services", err, logger.APILog)

			return
		}

		if notifier != nil {
			notifier.ProfileChanged(context.WithoutCancel(r.Context()), profile.ID)
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Profile IMS services updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateProfileIMSServicesAction, email, getClientIP(r), "Updated IMS services of profile "+profile.Name+" ["+strings.Join(params.Services, " ")+"]")
	})
}
