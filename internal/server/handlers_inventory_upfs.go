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

type CreateUPFParams struct {
	Name           string `json:"name"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type CreateUPFResponse struct {
	ID int64 `json:"id"`
}

type GetUPFResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	NetworkSliceId int64  `json:"network_slice_id"`
}

type DeleteUPFResponse struct {
	ID int64 `json:"id"`
}

func ListUPFs(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		upfs, err := env.DBQueries.ListUPFs(context.Background())
		if err != nil {
			log.Println("couldn't list UPFs: ", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(upfs))
		for _, upf := range upfs {
			ids = append(ids, upf.ID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateUPF(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var upf CreateUPFParams
		if err := json.NewDecoder(r.Body).Decode(&upf); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if upf.Name == "" {
			writeError(w, http.StatusBadRequest, "`name` is required")
			return
		}

		var networkSliceId sql.NullInt64
		if upf.NetworkSliceId != 0 {
			_, err := env.DBQueries.GetNetworkSlice(context.Background(), upf.NetworkSliceId)
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
				Int64: upf.NetworkSliceId,
				Valid: true,
			}
		}

		dbUPF := db.CreateUPFParams{
			Name:           upf.Name,
			NetworkSliceID: networkSliceId,
		}
		newUPF, err := env.DBQueries.CreateUPF(context.Background(), dbUPF)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateUPFResponse{ID: newUPF.ID}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetUPF(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		upf, err := env.DBQueries.GetUPF(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "UPF not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		upfResponse := GetUPFResponse{
			ID:             upf.ID,
			Name:           upf.Name,
			NetworkSliceId: upf.NetworkSliceID.Int64,
		}

		w.WriteHeader(http.StatusOK)
		err = writeJSON(w, upfResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteUPF(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		err = env.DBQueries.DeleteUPF(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteUPFResponse{ID: idInt64}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
