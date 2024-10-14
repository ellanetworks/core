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

type CreateSubscriberParams struct {
	IMSI           string `json:"imsi"`
	PLMNId         string `json:"plmn_id"`
	OPC            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequence_number"`
}

type CreateSubscriberResponse struct {
	ID int64 `json:"id"`
}

type UpdateSubscriberParams struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

type UpdateSubscriberResponse struct {
	ID int64 `json:"id"`
}

type GetSubscriberResponse struct {
	ID             int64  `json:"id"`
	IMSI           string `json:"imsi"`
	PLMNId         string `json:"plmn_id"`
	OPC            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequence_number"`
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
			writeError(w, http.StatusBadRequest, "IMSI is required")
			return
		}
		if subscriber.PLMNId == "" {
			writeError(w, http.StatusBadRequest, "PLMNId is required")
			return
		}
		if subscriber.OPC == "" {
			writeError(w, http.StatusBadRequest, "OPC is required")
			return
		}
		if subscriber.Key == "" {
			writeError(w, http.StatusBadRequest, "Key is required")
			return
		}
		if subscriber.SequenceNumber == "" {
			writeError(w, http.StatusBadRequest, "SequenceNumber is required")
			return
		}

		dbSubscriber := db.CreateSubscriberParams{
			Imsi:           subscriber.IMSI,
			PlmnID:         subscriber.PLMNId,
			Opc:            subscriber.OPC,
			Key:            subscriber.Key,
			SequenceNumber: subscriber.SequenceNumber,
		}
		newSubscriber, err := env.DBQueries.CreateSubscriber(context.Background(), dbSubscriber)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
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
		err = env.DBQueries.DeleteSubscriber(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
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
