package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
)

func pathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}

func getEmailFromContext(r *http.Request) string {
	if email, ok := r.Context().Value("email").(string); ok {
		return email
	}
	return ""
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, out any) error {
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeErrorHTTP(w, http.StatusBadRequest, "Invalid JSON body", err, logger.APILog)
		return err
	}
	return nil
}
