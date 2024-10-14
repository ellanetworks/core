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

type CreateDeviceGroupSubscriberParams struct {
	SubscriberID int64 `json:"subscriber_id"`
}

type CreateDeviceGroupSubscriberResponse struct {
	SubscriberID int64 `json:"subscriber_id"`
}

type DeleteDeviceGroupSubscriberResponse struct {
	SubscriberID int64 `json:"subscriber_id"`
}

func ListDeviceGroupSubscribers(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceGroupId := r.PathValue("device_group_id")
		deviceGroupIdInt64, err := strconv.ParseInt(deviceGroupId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
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

		deviceGroupSubscribers, err := env.DBQueries.ListDeviceGroupSubscribers(context.Background(), deviceGroupIdInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(deviceGroupSubscribers))
		for _, deviceGroupSubscriber := range deviceGroupSubscribers {
			ids = append(ids, deviceGroupSubscriber.SubscriberID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateDeviceGroupSubscriber(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		var deviceGroupSubscriber CreateDeviceGroupSubscriberParams
		if err := json.NewDecoder(r.Body).Decode(&deviceGroupSubscriber); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if deviceGroupSubscriber.SubscriberID == 0 {
			writeError(w, http.StatusBadRequest, "`subscriber_id` is required")
			return
		}

		_, err = env.DBQueries.GetSubscriber(context.Background(), deviceGroupSubscriber.SubscriberID)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "subscriber not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Check if the subscriber is already in the device group
		getDeviceGroupSubscriber := db.GetDeviceGroupSubscriberParams{
			DeviceGroupID: deviceGroupIdInt64,
			SubscriberID:  deviceGroupSubscriber.SubscriberID,
		}
		_, err = env.DBQueries.GetDeviceGroupSubscriber(context.Background(), getDeviceGroupSubscriber)
		if err == nil {
			writeError(w, http.StatusConflict, "subscriber already in device group")
			return
		}

		dbDeviceGroup := db.CreateDeviceGroupSubscriberParams{
			DeviceGroupID: deviceGroupIdInt64,
			SubscriberID:  deviceGroupSubscriber.SubscriberID,
		}
		err = env.DBQueries.CreateDeviceGroupSubscriber(context.Background(), dbDeviceGroup)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateDeviceGroupSubscriberResponse{
			SubscriberID: deviceGroupSubscriber.SubscriberID,
		}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteDeviceGroupSubscriber(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		subscriberId := r.PathValue("subscriber_id")
		subscriberIdInt64, err := strconv.ParseInt(subscriberId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "subscriber_id must be an integer")
			return
		}

		_, err = env.DBQueries.GetSubscriber(context.Background(), subscriberIdInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "subscriber not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		params := db.DeleteDeviceGroupSubscriberParams{
			DeviceGroupID: deviceGroupIdInt64,
			SubscriberID:  subscriberIdInt64,
		}
		err = env.DBQueries.DeleteDeviceGroupSubscriber(context.Background(), params)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteDeviceGroupSubscriberResponse{
			SubscriberID: subscriberIdInt64,
		}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
