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

type CreateRadioParams struct {
	Name           string `json:"name"`
	Tac            string `json:"tac"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type CreateRadioResponse struct {
	ID int64 `json:"id"`
}

type GetRadioResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Tac            string `json:"tac"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type DeleteRadioResponse struct {
	ID int64 `json:"id"`
}

func ListRadios(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		radios, err := env.DBQueries.ListRadios(context.Background())
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(radios))
		for _, radio := range radios {
			ids = append(ids, radio.ID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateRadio(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var radio CreateRadioParams
		if err := json.NewDecoder(r.Body).Decode(&radio); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if radio.Name == "" {
			writeError(w, http.StatusBadRequest, "`name` is required")
			return
		}
		if radio.Tac == "" {
			writeError(w, http.StatusBadRequest, "`tac` is required")
			return
		}

		var networkSliceId sql.NullInt64
		if radio.NetworkSliceId != 0 {
			_, err := env.DBQueries.GetNetworkSlice(context.Background(), radio.NetworkSliceId)
			if err != nil {
				if err == sql.ErrNoRows {
					writeError(w, http.StatusBadRequest, "network slice not found")
					return
				}
				log.Println(err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			networkSliceId = sql.NullInt64{
				Int64: radio.NetworkSliceId,
				Valid: true,
			}
		}

		dbRadio := db.CreateRadioParams{
			Name:           radio.Name,
			Tac:            radio.Tac,
			NetworkSliceID: networkSliceId,
		}
		newRadio, err := env.DBQueries.CreateRadio(context.Background(), dbRadio)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateRadioResponse{ID: newRadio.ID}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetRadio(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		radio, err := env.DBQueries.GetRadio(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "Radio not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		radioResponse := GetRadioResponse{
			ID:             radio.ID,
			Name:           radio.Name,
			Tac:            radio.Tac,
			NetworkSliceId: radio.NetworkSliceID.Int64,
		}

		w.WriteHeader(http.StatusOK)
		err = writeJSON(w, radioResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteRadio(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		err = env.DBQueries.DeleteRadio(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteRadioResponse{ID: idInt64}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
