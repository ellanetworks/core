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

type CreateNetworkSliceGnbParams struct {
	GnbID int64 `json:"gnb_id"`
}

type CreateNetworkSliceGnbResponse struct {
	GnbID int64 `json:"gnb_id"`
}

type DeleteNetworkSliceGnbResponse struct {
	GnbID int64 `json:"gnb_id"`
}

func ListNetworkSliceGnbs(env *HandlerConfig) http.HandlerFunc {
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

		networkSliceGnbs, err := env.DBQueries.ListNetworkSliceGnbs(context.Background(), networkSliceIdInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(networkSliceGnbs))
		for _, networkSliceGnb := range networkSliceGnbs {
			ids = append(ids, networkSliceGnb.GnbID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateNetworkSliceGnb(env *HandlerConfig) http.HandlerFunc {
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

		var networkSliceGnb CreateNetworkSliceGnbParams
		if err := json.NewDecoder(r.Body).Decode(&networkSliceGnb); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if networkSliceGnb.GnbID == 0 {
			writeError(w, http.StatusBadRequest, "`gnb_id` is required")
			return
		}

		_, err = env.DBQueries.GetGnb(context.Background(), networkSliceGnb.GnbID)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "gnb not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Check if the gnb is already in the network slice
		getNetworkSliceGnb := db.GetNetworkSliceGnbParams{
			NetworkSliceID: networkSliceIdInt64,
			GnbID:          networkSliceGnb.GnbID,
		}
		_, err = env.DBQueries.GetNetworkSliceGnb(context.Background(), getNetworkSliceGnb)
		if err == nil {
			writeError(w, http.StatusConflict, "gnb already in network slice")
			return
		}

		dbNetworkSlice := db.CreateNetworkSliceGnbParams{
			NetworkSliceID: networkSliceIdInt64,
			GnbID:          networkSliceGnb.GnbID,
		}
		err = env.DBQueries.CreateNetworkSliceGnb(context.Background(), dbNetworkSlice)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateNetworkSliceGnbResponse{
			GnbID: networkSliceGnb.GnbID,
		}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteNetworkSliceGnb(env *HandlerConfig) http.HandlerFunc {
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

		gnbId := r.PathValue("gnb_id")
		gnbIdInt64, err := strconv.ParseInt(gnbId, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "gnb_id must be an integer")
			return
		}

		_, err = env.DBQueries.GetGnb(context.Background(), gnbIdInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "gnb not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		params := db.DeleteNetworkSliceGnbParams{
			NetworkSliceID: networkSliceIdInt64,
			GnbID:          gnbIdInt64,
		}
		err = env.DBQueries.DeleteNetworkSliceGnb(context.Background(), params)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteNetworkSliceGnbResponse{
			GnbID: gnbIdInt64,
		}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
