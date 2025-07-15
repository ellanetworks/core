package server

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type CreateSubscriberParams struct {
	Imsi           string `json:"imsi"`
	Key            string `json:"key"`
	Opc            string `json:"opc,omitempty"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profileName"`
}

type UpdateSubscriberParams struct {
	Imsi        string `json:"imsi"`
	ProfileName string `json:"profileName"`
}

type GetSubscriberResponse struct {
	Imsi           string `json:"imsi"`
	IPAddress      string `json:"ipAddress"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
	Key            string `json:"key"`
	ProfileName    string `json:"profileName"`
}

const (
	ListSubscribersAction  = "list_subscribers"
	GetSubscriberAction    = "get_subscriber"
	CreateSubscriberAction = "create_subscriber"
	UpdateSubscriberAction = "update_subscriber"
	DeleteSubscriberAction = "delete_subscriber"
)

func isImsiValid(ctx context.Context, imsi string, dbInstance *db.Database) bool {
	if len(imsi) != 15 {
		return false
	}
	network, err := dbInstance.GetOperator(ctx)
	if err != nil {
		logger.APILog.Warn("Failed to retrieve operator", zap.Error(err))
		return false
	}
	Mcc := network.Mcc
	Mnc := network.Mnc
	if imsi[:3] != Mcc || imsi[3:5] != Mnc {
		return false
	}
	return true
}

func isHexString(input string) bool {
	_, err := hex.DecodeString(input)
	return err == nil
}

func isSequenceNumberValid(sequenceNumber string) bool {
	bytes, err := hex.DecodeString(sequenceNumber)
	if err != nil {
		return false
	}
	return len(bytes) == 6
}

func ListSubscribers(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		ctx := r.Context()
		dbSubscribers, err := dbInstance.ListSubscribers(ctx)
		if err != nil {
			logger.APILog.Warn("Failed to list subscribers", zap.Error(err))
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to list subscribers", err, logger.APILog)
			return
		}

		subscribers := make([]GetSubscriberResponse, 0, len(dbSubscribers))
		for _, dbSubscriber := range dbSubscribers {
			profile, err := dbInstance.GetProfileByID(ctx, dbSubscriber.ProfileID)
			if err != nil {
				writeErrorHTTP(w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)
				return
			}
			subscribers = append(subscribers, GetSubscriberResponse{
				Imsi:           dbSubscriber.Imsi,
				IPAddress:      dbSubscriber.IPAddress,
				Opc:            dbSubscriber.Opc,
				Key:            dbSubscriber.PermanentKey,
				SequenceNumber: dbSubscriber.SequenceNumber,
				ProfileName:    profile.Name,
			})
		}

		writeResponse(w, subscribers, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			ListSubscribersAction,
			email,
			r.RemoteAddr,
			"User listed subscribers",
		)
	})
}

func GetSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value("email").(string)
		if !ok {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		imsi := pathParam(r.URL.Path, "/api/v1/subscribers/") // extract "{imsi}" manually
		if imsi == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Subscriber not found", err, logger.APILog)
			return
		}
		profile, err := dbInstance.GetProfileByID(r.Context(), dbSubscriber.ProfileID)
		if err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)
			return
		}

		subscriber := GetSubscriberResponse{
			Imsi:           dbSubscriber.Imsi,
			IPAddress:      dbSubscriber.IPAddress,
			Opc:            dbSubscriber.Opc,
			SequenceNumber: dbSubscriber.SequenceNumber,
			Key:            dbSubscriber.PermanentKey,
			ProfileName:    profile.Name,
		}
		writeResponse(w, subscriber, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(GetSubscriberAction, email, r.RemoteAddr, "User retrieved subscriber: "+imsi)
	})
}

func CreateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		var params CreateSubscriberParams
		if err := decodeJSONBody(w, r, &params); err != nil {
			return // already written
		}

		if params.Imsi == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.ProfileName == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing profileName parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.SequenceNumber == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing sequenceNumber parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if !isImsiValid(r.Context(), params.Imsi, dbInstance) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.", errors.New("validation error"), logger.APILog)
			return
		}

		if !isSequenceNumberValid(params.SequenceNumber) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if !isHexString(params.Key) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid key format. Must be a 32-character hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if params.Opc != "" && !isHexString(params.Opc) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid OPC format. Must be a 32-character hex string.", errors.New("validation error"), logger.APILog)
			return
		}

		keyBytes, _ := hex.DecodeString(params.Key)
		opcHex := params.Opc
		if opcHex == "" {
			operatorCode, err := dbInstance.GetOperatorCode(r.Context())
			if err != nil {
				writeErrorHTTP(w, http.StatusInternalServerError, "Failed to get operator code", err, logger.APILog)
				return
			}
			opBytes, _ := hex.DecodeString(operatorCode)
			derivedOPC, _ := deriveOPc(keyBytes, opBytes)
			opcHex = hex.EncodeToString(derivedOPC)
		}

		if _, err := dbInstance.GetSubscriber(r.Context(), params.Imsi); err == nil {
			writeErrorHTTP(w, http.StatusBadRequest, "Subscriber already exists", errors.New("duplicate"), logger.APILog)
			return
		}

		profile, err := dbInstance.GetProfile(r.Context(), params.ProfileName)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Profile not found", err, logger.APILog)
			return
		}

		newSubscriber := &db.Subscriber{
			Imsi:           params.Imsi,
			SequenceNumber: params.SequenceNumber,
			PermanentKey:   params.Key,
			Opc:            opcHex,
			ProfileID:      profile.ID,
		}
		if err := dbInstance.CreateSubscriber(r.Context(), newSubscriber); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to create subscriber", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber created successfully"}, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(CreateSubscriberAction, email, r.RemoteAddr, "User created subscriber: "+params.Imsi)
	})
}

func UpdateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		imsi := pathParam(r.URL.Path, "/api/v1/subscribers/")
		if imsi == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		var params UpdateSubscriberParams
		if err := decodeJSONBody(w, r, &params); err != nil {
			return // already written
		}

		if params.Imsi == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.ProfileName == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing profileName parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if !isImsiValid(r.Context(), params.Imsi, dbInstance) {
			writeErrorHTTP(w, http.StatusBadRequest, "Invalid IMSI", errors.New("validation error"), logger.APILog)
			return
		}

		existing, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Subscriber not found", err, logger.APILog)
			return
		}
		profile, err := dbInstance.GetProfile(r.Context(), params.ProfileName)
		if err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Profile not found", err, logger.APILog)
			return
		}

		updated := &db.Subscriber{
			Imsi:           existing.Imsi,
			SequenceNumber: existing.SequenceNumber,
			PermanentKey:   existing.PermanentKey,
			Opc:            existing.Opc,
			ProfileID:      profile.ID,
		}
		if err := dbInstance.UpdateSubscriber(r.Context(), updated); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to update subscriber", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateSubscriberAction, email, r.RemoteAddr, "User updated subscriber: "+imsi)
	})
}

func DeleteSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		imsi := pathParam(r.URL.Path, "/api/v1/subscribers/")
		if imsi == "" {
			writeErrorHTTP(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}
		if _, err := dbInstance.GetSubscriber(r.Context(), imsi); err != nil {
			writeErrorHTTP(w, http.StatusNotFound, "Subscriber not found", err, logger.APILog)
			return
		}
		if err := dbInstance.DeleteSubscriber(r.Context(), imsi); err != nil {
			writeErrorHTTP(w, http.StatusInternalServerError, "Failed to delete subscriber", err, logger.APILog)
			return
		}
		writeResponse(w, SuccessResponse{Message: "Subscriber deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeleteSubscriberAction, email, r.RemoteAddr, "User deleted subscriber: "+imsi)
	})
}
