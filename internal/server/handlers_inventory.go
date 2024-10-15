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

type CreateGnbParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type CreateGnbResponse struct {
	ID int64 `json:"id"`
}

type GetGnbResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type DeleteGnbResponse struct {
	ID int64 `json:"id"`
}

func ListGnbs(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gnbs, err := env.DBQueries.ListGnbs(context.Background())
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids := make([]int64, 0, len(gnbs))
		for _, gnb := range gnbs {
			ids = append(ids, gnb.ID)
		}

		err = writeJSON(w, ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateGnb(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var gnb CreateGnbParams
		if err := json.NewDecoder(r.Body).Decode(&gnb); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		if gnb.Name == "" {
			writeError(w, http.StatusBadRequest, "`name` is required")
			return
		}
		if gnb.Tac == "" {
			writeError(w, http.StatusBadRequest, "`tac` is required")
			return
		}

		dbGnb := db.CreateGnbParams{
			Name: gnb.Name,
			Tac:  gnb.Tac,
		}
		newGnb, err := env.DBQueries.CreateGnb(context.Background(), dbGnb)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := CreateGnbResponse{ID: newGnb.ID}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetGnb(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}

		gnb, err := env.DBQueries.GetGnb(context.Background(), idInt64)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "Gnb not found")
				return
			}
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		gnbResponse := GetGnbResponse{
			ID:   gnb.ID,
			Name: gnb.Name,
			Tac:  gnb.Tac,
		}

		w.WriteHeader(http.StatusOK)
		err = writeJSON(w, gnbResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteGnb(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		idInt64, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		err = env.DBQueries.DeleteGnb(context.Background(), idInt64)
		if err != nil {
			log.Println(err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		response := DeleteGnbResponse{ID: idInt64}
		err = writeJSON(w, response)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
