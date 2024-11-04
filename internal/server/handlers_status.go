package server

import (
	"net/http"

	"github.com/yeastengine/ella/version"
)

type StatusResponse struct {
	Version string `json:"version"`
}

// GetStatus returns the version of the server
func GetStatus(env *HandlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusResponse := StatusResponse{
			Version: version.GetVersion(),
		}
		w.WriteHeader(http.StatusOK)
		err := writeJSON(w, statusResponse)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
