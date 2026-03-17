package server

import (
	"crypto/ecdh"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreateHomeNetworkKeyParams struct {
	KeyIdentifier int    `json:"keyIdentifier"`
	Scheme        string `json:"scheme"`
	PrivateKey    string `json:"privateKey"`
}

type HomeNetworkKeyResponse struct {
	ID            int    `json:"id"`
	KeyIdentifier int    `json:"keyIdentifier"`
	Scheme        string `json:"scheme"`
	PublicKey     string `json:"publicKey"`
}

type HomeNetworkKeyPrivateKeyResponse struct {
	PrivateKey string `json:"privateKey"`
}

const (
	CreateHomeNetworkKeyAction         = "create_home_network_key"
	DeleteHomeNetworkKeyAction         = "delete_home_network_key"
	ViewHomeNetworkKeyPrivateKeyAction = "view_home_network_key_private_key"
)

func isValidHomeNetworkScheme(scheme string) bool {
	return scheme == "A" || scheme == "B"
}

func isValidHomeNetworkPrivateKey(scheme, privateKey string) bool {
	privBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return false
	}

	switch scheme {
	case "A":
		_, err = ecdh.X25519().NewPrivateKey(privBytes)
	case "B":
		_, err = ecdh.P256().NewPrivateKey(privBytes)
	default:
		return false
	}

	return err == nil
}

func GetHomeNetworkKeyPrivateKey(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		idStr := r.PathValue("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid key ID", err, logger.APILog)
			return
		}

		existingKey, err := dbInstance.GetHomeNetworkKey(r.Context(), id)
		if err != nil {
			if err == db.ErrNotFound {
				writeError(r.Context(), w, http.StatusNotFound, "Home network key not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get home network key", err, logger.APILog)

			return
		}

		logger.LogAuditEvent(
			r.Context(),
			ViewHomeNetworkKeyPrivateKeyAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Viewed private key for home network key (id=%d, scheme=%s, keyIdentifier=%d)", id, existingKey.Scheme, existingKey.KeyIdentifier),
		)

		writeResponse(r.Context(), w, HomeNetworkKeyPrivateKeyResponse{
			PrivateKey: existingKey.PrivateKey,
		}, http.StatusOK, logger.APILog)
	})
}

func CreateHomeNetworkKey(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateHomeNetworkKeyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if !isValidHomeNetworkScheme(params.Scheme) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid scheme. Must be \"A\" or \"B\".", nil, logger.APILog)
			return
		}

		if params.KeyIdentifier < 0 || params.KeyIdentifier > 255 {
			writeError(r.Context(), w, http.StatusBadRequest, "keyIdentifier must be between 0 and 255", nil, logger.APILog)
			return
		}

		if params.PrivateKey == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "privateKey is missing", nil, logger.APILog)
			return
		}

		if !isValidHomeNetworkPrivateKey(params.Scheme, params.PrivateKey) {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("Invalid private key for scheme %s", params.Scheme), nil, logger.APILog)

			return
		}

		count, err := dbInstance.CountHomeNetworkKeys(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count home network keys", err, logger.APILog)
			return
		}

		if count >= db.MaxHomeNetworkKeys {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("Maximum number of home network keys (%d) reached", db.MaxHomeNetworkKeys), nil, logger.APILog)

			return
		}

		key := &db.HomeNetworkKey{
			KeyIdentifier: params.KeyIdentifier,
			Scheme:        params.Scheme,
			PrivateKey:    params.PrivateKey,
		}

		if err := dbInstance.CreateHomeNetworkKey(r.Context(), key); err != nil {
			if err == db.ErrAlreadyExists {
				writeError(r.Context(), w, http.StatusConflict,
					fmt.Sprintf("A key with scheme=%s and keyIdentifier=%d already exists", params.Scheme, params.KeyIdentifier), nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create home network key", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Home network key created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			CreateHomeNetworkKeyAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Created home network key (scheme=%s, keyIdentifier=%d)", params.Scheme, params.KeyIdentifier),
		)
	})
}

func DeleteHomeNetworkKey(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		idStr := r.PathValue("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid key ID", err, logger.APILog)
			return
		}

		existingKey, err := dbInstance.GetHomeNetworkKey(r.Context(), id)
		if err != nil {
			if err == db.ErrNotFound {
				writeError(r.Context(), w, http.StatusNotFound, "Home network key not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get home network key", err, logger.APILog)

			return
		}

		if err := dbInstance.DeleteHomeNetworkKey(r.Context(), id); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete home network key", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Home network key deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			DeleteHomeNetworkKeyAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Deleted home network key (id=%d, scheme=%s, keyIdentifier=%d)", id, existingKey.Scheme, existingKey.KeyIdentifier),
		)
	})
}
