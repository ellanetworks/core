package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/omec-project/openapi/models"
	db "github.com/yeastengine/ella/internal/db/sql"

	"github.com/yeastengine/ella/internal/amf/gmm"
	"github.com/yeastengine/ella/internal/udr/producer"
)

type CreateSubscriberParams struct {
	IMSI           string `json:"imsi"`
	PLMNId         string `json:"plmn_id"`
	OPC            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequence_number"`
	DeviceGroupId  int64  `json:"device_group_id"`
}

type CreateSubscriberResponse struct {
	ID int64 `json:"id"`
}

type GetSubscriberResponse struct {
	ID             int64  `json:"id"`
	IMSI           string `json:"imsi"`
	PLMNId         string `json:"plmn_id"`
	OPC            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequence_number"`
	DeviceGroupId  int64  `json:"device_group_id"`
}

type DeleteSubscriberResponse struct {
	ID int64 `json:"id"`
}

func ListSubscribers(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subscribers, err := env.DBQueries.ListSubscribers(context.Background())
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(subscribers))
		for _, subscriber := range subscribers {
			ids = append(ids, subscriber.ID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateSubscriber(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var subscriber CreateSubscriberParams
		if err := json.NewDecoder(r.Body).Decode(&subscriber); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if subscriber.IMSI == "" {
			writeError(w, http.StatusBadRequest, "`imsi` is required")
			return
		}
		if subscriber.PLMNId == "" {
			writeError(w, http.StatusBadRequest, "`plmn_id` is required")
			return
		}
		if subscriber.OPC == "" {
			writeError(w, http.StatusBadRequest, "`opc` is required")
			return
		}
		if subscriber.Key == "" {
			writeError(w, http.StatusBadRequest, "`key` is required")
			return
		}
		if subscriber.SequenceNumber == "" {
			writeError(w, http.StatusBadRequest, "`sequence_number` is required")
			return
		}
		if subscriber.DeviceGroupId <= 0 {
			writeError(w, http.StatusBadRequest, "`device_group_id` is required")
			return
		}

		var deviceGroupId sql.NullInt64
		if subscriber.DeviceGroupId != 0 {
			_, err := env.DBQueries.GetDeviceGroup(context.Background(), subscriber.DeviceGroupId)
			if err != nil {
				if err == sql.ErrNoRows {
					writeError(w, http.StatusBadRequest, "device group not found")
					return
				}
				log.Println(err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			deviceGroupId = sql.NullInt64{Int64: subscriber.DeviceGroupId, Valid: true}
		}

		dbSubscriber := db.CreateSubscriberParams{
			Imsi:           subscriber.IMSI,
			PlmnID:         subscriber.PLMNId,
			Opc:            subscriber.OPC,
			Key:            subscriber.Key,
			SequenceNumber: subscriber.SequenceNumber,
			DeviceGroupID:  deviceGroupId,
		}
		newSubscriber, err := env.DBQueries.CreateSubscriber(context.Background(), dbSubscriber)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		err = addSubscriberToUDR(env.DBQueries, newSubscriber)
		if err != nil {
			log.Println("couldn't add subscriber to UDR: ", err)
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateSubscriberResponse{ID: newSubscriber.ID}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func addSubscriberToUDR(queries *db.Queries, subscriber db.Subscriber) error {
	deviceGroup, err := queries.GetDeviceGroup(context.Background(), subscriber.DeviceGroupID.Int64)
	if err != nil {
		return err
	}
	networkSlice, err := queries.GetNetworkSlice(context.Background(), deviceGroup.NetworkSliceID)
	if err != nil {
		return err
	}
	snssai := models.Snssai{
		Sst: int32(networkSlice.Sst),
		Sd:  networkSlice.Sd,
	}
	err = producer.AddEntrySmPolicyTable(subscriber.Imsi, deviceGroup.Dnn, snssai)
	if err != nil {
		return fmt.Errorf("couldn't add entry in SM policy table: %w", err)
	}
	return nil
}

func deregisterSubscriberFromAMF(queries *db.Queries, subscriber db.Subscriber) error {
	deviceGroup, err := queries.GetDeviceGroup(context.Background(), subscriber.DeviceGroupID.Int64)
	if err != nil {
		return err
	}
	networkSlice, err := queries.GetNetworkSlice(context.Background(), deviceGroup.NetworkSliceID)
	if err != nil {
		return err
	}
	snssai := models.Snssai{
		Sst: int32(networkSlice.Sst),
		Sd:  networkSlice.Sd,
	}
	gmm.SendDeregistrationMessage(subscriber.Imsi, snssai)
	return nil
}

func GetSubscriber(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		subscriber, err := env.DBQueries.GetSubscriber(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "Subscriber not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		subscriberResponse := GetSubscriberResponse{
			ID:             subscriber.ID,
			IMSI:           subscriber.Imsi,
			PLMNId:         subscriber.PlmnID,
			OPC:            subscriber.Opc,
			Key:            subscriber.Key,
			SequenceNumber: subscriber.SequenceNumber,
			DeviceGroupId:  subscriber.DeviceGroupID.Int64,
		}

		w.WriteHeader(http.StatusOK)
		err = writeJSON(w, subscriberResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteSubscriber(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		subscriber, err := env.DBQueries.GetSubscriber(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "Subscriber not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		err = env.DBQueries.DeleteSubscriber(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		err = deregisterSubscriberFromAMF(env.DBQueries, subscriber)
		if err != nil {
			log.Println("couldn't deregister subscriber from AMF: ", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteSubscriberResponse{ID: idInt64}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
