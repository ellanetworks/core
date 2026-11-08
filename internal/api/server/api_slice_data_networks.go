// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

type SliceDataNetwork struct {
	DataNetwork     string `json:"data_network"`
	NAT             bool   `json:"nat"`
	LocalSwitch     bool   `json:"local_switch"`
	NetworkInstance string `json:"network_instance"`
}

type CreateSliceDataNetworkParams struct {
	DataNetwork     string `json:"data_network"`
	NAT             *bool  `json:"nat,omitempty"`
	LocalSwitch     *bool  `json:"local_switch,omitempty"`
	NetworkInstance string `json:"network_instance,omitempty"`
}

type UpdateSliceDataNetworkParams struct {
	NAT             *bool   `json:"nat,omitempty"`
	LocalSwitch     *bool   `json:"local_switch,omitempty"`
	NetworkInstance *string `json:"network_instance,omitempty"`
}

type ListSliceDataNetworksResponse struct {
	Items []SliceDataNetwork `json:"items"`
}

const (
	CreateSliceDataNetworkAction = "create_slice_data_network"
	UpdateSliceDataNetworkAction = "update_slice_data_network"
	DeleteSliceDataNetworkAction = "delete_slice_data_network"
)

func ListSliceDataNetworks(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slice, ok := pathSlice(w, r, dbInstance)
		if !ok {
			return
		}

		forwarding, err := dbInstance.ListSliceDataNetworkForwarding(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list the data networks of the slice", err, logger.APILog)
			return
		}

		names, err := dataNetworkNames(r.Context(), dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list data networks", err, logger.APILog)
			return
		}

		items := make([]SliceDataNetwork, 0)

		for _, f := range forwarding {
			if f.SliceID == slice.ID {
				items = append(items, sliceDataNetworkResponse(f, names[f.DataNetworkID]))
			}
		}

		writeResponse(r.Context(), w, ListSliceDataNetworksResponse{Items: items}, http.StatusOK, logger.APILog)
	})
}

func GetSliceDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slice, dataNetwork, ok := pathSliceDataNetwork(w, r, dbInstance)
		if !ok {
			return
		}

		f, err := dbInstance.GetSliceDataNetworkForwarding(r.Context(), slice.ID, dataNetwork.ID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "The slice does not serve this data network", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the slice data network", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, sliceDataNetworkResponse(*f, dataNetwork.Name), http.StatusOK, logger.APILog)
	})
}

func CreateSliceDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		slice, ok := pathSlice(w, r, dbInstance)
		if !ok {
			return
		}

		var params CreateSliceDataNetworkParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.DataNetwork == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "data_network is missing", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), params.DataNetwork)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		if err := validateIMSForwarding(dataNetwork.IMS, params.NAT, params.LocalSwitch); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := validateNetworkInstance(params.NetworkInstance); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		framed, err := dataNetworkHasFramedRoutes(r.Context(), dbInstance, dataNetwork.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check framed routes", err, logger.APILog)
			return
		}

		if framed && params.NAT != nil && *params.NAT {
			writeError(r.Context(), w, http.StatusConflict, "NAT cannot be enabled on a data network with framed routes", nil, logger.APILog)
			return
		}

		pair := &db.SliceDataNetwork{
			SliceID:         slice.ID,
			DataNetworkID:   dataNetwork.ID,
			NAT:             boolOr(params.NAT, db.NATDefaultEnabled && !framed) && !dataNetwork.IMS,
			LocalSwitch:     boolOr(params.LocalSwitch, dataNetwork.IMS),
			NetworkInstance: params.NetworkInstance,
		}

		if err := dbInstance.CreateSliceDataNetwork(r.Context(), pair); err != nil {
			writeSliceDataNetworkError(r.Context(), w, err, "Failed to serve the data network on the slice")
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "The slice now serves the data network"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateSliceDataNetworkAction, email, getClientIP(r), fmt.Sprintf("User made slice %s serve data network %s", slice.Name, dataNetwork.Name))
	})
}

func UpdateSliceDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		slice, dataNetwork, ok := pathSliceDataNetwork(w, r, dbInstance)
		if !ok {
			return
		}

		var params UpdateSliceDataNetworkParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateIMSForwarding(dataNetwork.IMS, params.NAT, params.LocalSwitch); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if params.NetworkInstance != nil {
			if err := validateNetworkInstance(*params.NetworkInstance); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
				return
			}
		}

		existing, err := dbInstance.GetSliceDataNetwork(r.Context(), slice.ID, dataNetwork.ID)
		if err != nil {
			writeSliceDataNetworkError(r.Context(), w, err, "Failed to get the slice data network")
			return
		}

		if params.NAT != nil && *params.NAT {
			framed, err := dataNetworkHasFramedRoutes(r.Context(), dbInstance, dataNetwork.ID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check framed routes", err, logger.APILog)
				return
			}

			if framed {
				writeError(r.Context(), w, http.StatusConflict, "NAT cannot be enabled on a data network with framed routes", nil, logger.APILog)
				return
			}
		}

		changed := false

		if params.NAT != nil && *params.NAT != existing.NAT {
			existing.NAT = *params.NAT
			changed = true
		}

		if params.LocalSwitch != nil && *params.LocalSwitch != existing.LocalSwitch {
			existing.LocalSwitch = *params.LocalSwitch
			changed = true
		}

		if params.NetworkInstance != nil && networkInstanceOrDefault(*params.NetworkInstance) != existing.NetworkInstance {
			existing.NetworkInstance = networkInstanceOrDefault(*params.NetworkInstance)
			changed = true
		}

		if changed {
			if err := dbInstance.UpdateSliceDataNetwork(r.Context(), existing); err != nil {
				writeSliceDataNetworkError(r.Context(), w, err, "Failed to update the slice data network")
				return
			}
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Slice data network updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateSliceDataNetworkAction, email, getClientIP(r), fmt.Sprintf("User updated data network %s on slice %s", dataNetwork.Name, slice.Name))
	})
}

func DeleteSliceDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		slice, dataNetwork, ok := pathSliceDataNetwork(w, r, dbInstance)
		if !ok {
			return
		}

		if err := dbInstance.DeleteSliceDataNetwork(r.Context(), slice.ID, dataNetwork.ID); err != nil {
			writeSliceDataNetworkError(r.Context(), w, err, "Failed to stop serving the data network on the slice")
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "The slice no longer serves the data network"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSliceDataNetworkAction, email, getClientIP(r), fmt.Sprintf("User made slice %s stop serving data network %s", slice.Name, dataNetwork.Name))
	})
}

func checkSliceServesDataNetwork(ctx context.Context, dbInstance *db.Database, slice *db.NetworkSlice, dataNetwork *db.DataNetwork) (int, error) {
	f, err := dbInstance.GetSliceDataNetworkForwarding(ctx, slice.ID, dataNetwork.ID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return http.StatusConflict, fmt.Errorf("slice %s does not serve data network %s", slice.Name, dataNetwork.Name)
		}

		return http.StatusInternalServerError, errors.New("failed to check the data networks of the slice")
	}

	if !f.NAT {
		return 0, nil
	}

	framed, err := dataNetworkHasFramedRoutes(ctx, dbInstance, dataNetwork.ID)
	if err != nil {
		return http.StatusInternalServerError, errors.New("failed to check framed routes")
	}

	if framed {
		return http.StatusConflict, fmt.Errorf("slice %s NATs data network %s, which has framed routes", slice.Name, dataNetwork.Name)
	}

	return 0, nil
}

func validateIMSForwarding(ims bool, nat, localSwitch *bool) error {
	if !ims {
		return nil
	}

	if nat != nil && *nat {
		return errors.New("an IMS data network cannot use NAT: the IMS must reach phones at their own address")
	}

	if localSwitch != nil && !*localSwitch {
		return errors.New("an IMS data network always switches locally")
	}

	return nil
}

func dataNetworkHasFramedRoutes(ctx context.Context, dbInstance *db.Database, dataNetworkID string) (bool, error) {
	framed, err := dbInstance.ListAllFramedRoutes(ctx)
	if err != nil {
		return false, err
	}

	for _, f := range framed {
		if f.DataNetworkID == dataNetworkID {
			return true, nil
		}
	}

	return false, nil
}

func dataNetworkNames(ctx context.Context, dbInstance *db.Database) (map[string]string, error) {
	dataNetworks, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		return nil, err
	}

	names := make(map[string]string, len(dataNetworks))
	for _, dn := range dataNetworks {
		names[dn.ID] = dn.Name
	}

	return names, nil
}

func sliceDataNetworkResponse(f db.SliceDataNetworkForwarding, name string) SliceDataNetwork {
	return SliceDataNetwork{DataNetwork: name, NAT: f.NAT, LocalSwitch: f.LocalSwitch, NetworkInstance: f.NetworkInstance}
}

func validateNetworkInstance(name string) error {
	if name == "" || models.ValidNetworkInstanceName(name) {
		return nil
	}

	return fmt.Errorf("invalid network_instance %q: lowercase labels of letters, digits and hyphens, at most 62 characters", name)
}

func networkInstanceOrDefault(name string) string {
	if name == "" {
		return models.DefaultNetworkInstanceName
	}

	return name
}

func pathSlice(w http.ResponseWriter, r *http.Request, dbInstance *db.Database) (*db.NetworkSlice, bool) {
	slice, err := dbInstance.GetNetworkSlice(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
		return nil, false
	}

	return slice, true
}

func pathSliceDataNetwork(w http.ResponseWriter, r *http.Request, dbInstance *db.Database) (*db.NetworkSlice, *db.DataNetwork, bool) {
	slice, ok := pathSlice(w, r, dbInstance)
	if !ok {
		return nil, nil, false
	}

	dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), r.PathValue("data_network"))
	if err != nil {
		writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
		return nil, nil, false
	}

	return slice, dataNetwork, true
}

func writeSliceDataNetworkError(ctx context.Context, w http.ResponseWriter, err error, msg string) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(ctx, w, http.StatusNotFound, "The slice does not serve this data network", nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(ctx, w, http.StatusConflict, "The slice already serves this data network", nil, logger.APILog)
	case errors.Is(err, db.ErrInUse):
		writeError(ctx, w, http.StatusConflict, "Policies use this data network on this slice", nil, logger.APILog)
	case errors.Is(err, db.ErrMigrationPending):
		writeError(ctx, w, http.StatusServiceUnavailable, "cluster is upgrading; feature unavailable until schema migration completes", nil, logger.APILog)
	default:
		writeError(ctx, w, http.StatusInternalServerError, msg, err, logger.APILog)
	}
}
