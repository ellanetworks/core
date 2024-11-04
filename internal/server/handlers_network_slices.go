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

type CreateNetworkSliceParams struct {
	Name     string `json:"name"`
	Sst      int32  `json:"sst"`
	Sd       string `json:"sd"`
	SiteName string `json:"site_name"`
	Mcc      string `json:"mcc"`
	Mnc      string `json:"mnc"`
}

type CreateNetworkSliceResponse struct {
	ID int64 `json:"id"`
}

type GetNetworkSliceResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Sst      int32  `json:"sst"`
	Sd       string `json:"sd"`
	SiteName string `json:"site_name"`
	Mcc      string `json:"mcc"`
	Mnc      string `json:"mnc"`
}

type DeleteNetworkSliceResponse struct {
	ID int64 `json:"id"`
}

func ListNetworkSlices(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkSlices, err := env.DBQueries.ListNetworkSlices(context.Background())
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(networkSlices))
		for _, networkSlice := range networkSlices {
			ids = append(ids, networkSlice.ID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateNetworkSlice(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var networkSlice CreateNetworkSliceParams
		if err := json.NewDecoder(r.Body).Decode(&networkSlice); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if networkSlice.Name == "" {
			writeError(w, http.StatusBadRequest, "`name` is required")
			return
		}
		if networkSlice.Sst == 0 {
			writeError(w, http.StatusBadRequest, "`sst` is required")
			return
		}
		if networkSlice.Sd == "" {
			writeError(w, http.StatusBadRequest, "`sd` is required")
			return
		}
		if networkSlice.SiteName == "" {
			writeError(w, http.StatusBadRequest, "`site_name` is required")
			return
		}
		if networkSlice.Mcc == "" {
			writeError(w, http.StatusBadRequest, "`mcc` is required")
			return
		}
		if networkSlice.Mnc == "" {
			writeError(w, http.StatusBadRequest, "`mnc` is required")
			return
		}

		dbNetworkSlice := db.CreateNetworkSliceParams{
			Name:     networkSlice.Name,
			Sst:      int64(networkSlice.Sst),
			Sd:       networkSlice.Sd,
			SiteName: networkSlice.SiteName,
			Mcc:      networkSlice.Mcc,
			Mnc:      networkSlice.Mnc,
		}
		newNetworkSlice, err := env.DBQueries.CreateNetworkSlice(context.Background(), dbNetworkSlice)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateNetworkSliceResponse{ID: newNetworkSlice.ID}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetNetworkSlice(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		networkSlice, err := env.DBQueries.GetNetworkSlice(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "NetworkSlice not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		networkSliceResponse := GetNetworkSliceResponse{
			ID:       networkSlice.ID,
			Name:     networkSlice.Name,
			Sst:      int32(networkSlice.Sst),
			Sd:       networkSlice.Sd,
			SiteName: networkSlice.SiteName,
			Mcc:      networkSlice.Mcc,
			Mnc:      networkSlice.Mnc,
		}

		w.WriteHeader(http.StatusOK)
		err = writeJSON(w, networkSliceResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteNetworkSlice(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		err = env.DBQueries.DeleteNetworkSlice(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteNetworkSliceResponse{ID: idInt64}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
