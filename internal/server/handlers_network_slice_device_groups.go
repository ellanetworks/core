package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	db "github.com/yeastengine/ella/internal/db/sql"
)

type CreateNetworkSliceDeviceGroupParams struct {
	DeviceGroupID int64 `json:"device_group_id"`
}

type CreateNetworkSliceDeviceGroupResponse struct {
	DeviceGroupID int64 `json:"device_group_id"`
}

type DeleteNetworkSliceDeviceGroupResponse struct {
	DeviceGroupID int64 `json:"device_group_id"`
}

func ListNetworkSliceDeviceGroups(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkSliceId := r.PathValue("network_slice_id")
		networkSliceIdInt64, err := strconv.ParseInt(networkSliceId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		_, err = env.DBQueries.GetNetworkSlice(context.Background(), networkSliceIdInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "network slice not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		networkSliceDeviceGroups, err := env.DBQueries.ListNetworkSliceDeviceGroups(context.Background(), networkSliceIdInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(networkSliceDeviceGroups))
		for _, networkSliceDeviceGroup := range networkSliceDeviceGroups {
			ids = append(ids, networkSliceDeviceGroup.DeviceGroupID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateNetworkSliceDeviceGroup(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkSliceId := r.PathValue("network_slice_id")
		networkSliceIdInt64, err := strconv.ParseInt(networkSliceId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "network_slice_id must be an integer")
			return
		}
		_, err = env.DBQueries.GetNetworkSlice(context.Background(), networkSliceIdInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "network slice not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		var networkSliceDeviceGroup CreateNetworkSliceDeviceGroupParams
		if err := json.NewDecoder(r.Body).Decode(&networkSliceDeviceGroup); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if networkSliceDeviceGroup.DeviceGroupID == 0 {
			writeError(w, http.StatusBadRequest, "`device_group_id` is required")
			return
		}

		_, err = env.DBQueries.GetDeviceGroup(context.Background(), networkSliceDeviceGroup.DeviceGroupID)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "device group not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Check if the device group is already in the network slice
		getNetworkSliceDeviceGroup := db.GetNetworkSliceDeviceGroupParams{
			NetworkSliceID: networkSliceIdInt64,
			DeviceGroupID:  networkSliceDeviceGroup.DeviceGroupID,
		}
		_, err = env.DBQueries.GetNetworkSliceDeviceGroup(context.Background(), getNetworkSliceDeviceGroup)
		if err == nil {
			writeError(w, http.StatusConflict, "device group already in network slice")
			return
		}

		dbNetworkSlice := db.CreateNetworkSliceDeviceGroupParams{
			NetworkSliceID: networkSliceIdInt64,
			DeviceGroupID:  networkSliceDeviceGroup.DeviceGroupID,
		}
		err = env.DBQueries.CreateNetworkSliceDeviceGroup(context.Background(), dbNetworkSlice)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateNetworkSliceDeviceGroupResponse{
			DeviceGroupID: networkSliceDeviceGroup.DeviceGroupID,
		}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteNetworkSliceDeviceGroup(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkSliceId := r.PathValue("network_slice_id")
		networkSliceIdInt64, err := strconv.ParseInt(networkSliceId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "network_slice_id must be an integer")
			return
		}

		_, err = env.DBQueries.GetNetworkSlice(context.Background(), networkSliceIdInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "network slice not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		deviceGroupId := r.PathValue("device_group_id")
		deviceGroupIdInt64, err := strconv.ParseInt(deviceGroupId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "device_group_id must be an integer")
			return
		}

		_, err = env.DBQueries.GetDeviceGroup(context.Background(), deviceGroupIdInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "device group not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		params := db.DeleteNetworkSliceDeviceGroupParams{
			NetworkSliceID: networkSliceIdInt64,
			DeviceGroupID:  deviceGroupIdInt64,
		}
		err = env.DBQueries.DeleteNetworkSliceDeviceGroup(context.Background(), params)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteNetworkSliceDeviceGroupResponse{
			DeviceGroupID: deviceGroupIdInt64,
		}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
