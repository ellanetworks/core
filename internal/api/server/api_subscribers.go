package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	PolicyName     string `json:"policyName"`
}

type UpdateSubscriberParams struct {
	Imsi       string `json:"imsi"`
	PolicyName string `json:"policyName"`
}

type GetSubscriberResponse struct {
	Imsi           string `json:"imsi"`
	IPAddress      string `json:"ipAddress"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
	Key            string `json:"key"`
	PolicyName     string `json:"policyName"`
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
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		ctx := r.Context()
		dbSubscribers, err := dbInstance.ListSubscribers(ctx)
		if err != nil {
			logger.APILog.Warn("Failed to list subscribers", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to list subscribers", err, logger.APILog)
			return
		}

		subscribers := make([]GetSubscriberResponse, 0, len(dbSubscribers))
		for _, dbSubscriber := range dbSubscribers {
			policy, err := dbInstance.GetPolicyByID(ctx, dbSubscriber.PolicyID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}
			subscribers = append(subscribers, GetSubscriberResponse{
				Imsi:           dbSubscriber.Imsi,
				IPAddress:      dbSubscriber.IPAddress,
				Opc:            dbSubscriber.Opc,
				Key:            dbSubscriber.PermanentKey,
				SequenceNumber: dbSubscriber.SequenceNumber,
				PolicyName:     policy.Name,
			})
		}

		writeResponse(w, subscribers, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			ListSubscribersAction,
			email,
			getClientIP(r),
			"User listed subscribers",
		)
	})
}

func GetSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		imsi := pathParam(r.URL.Path, "/api/v1/subscribers/") // extract "{imsi}" manually
		if imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			writeError(w, http.StatusNotFound, "Subscriber not found", err, logger.APILog)
			return
		}
		policy, err := dbInstance.GetPolicyByID(r.Context(), dbSubscriber.PolicyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		subscriber := GetSubscriberResponse{
			Imsi:           dbSubscriber.Imsi,
			IPAddress:      dbSubscriber.IPAddress,
			Opc:            dbSubscriber.Opc,
			SequenceNumber: dbSubscriber.SequenceNumber,
			Key:            dbSubscriber.PermanentKey,
			PolicyName:     policy.Name,
		}
		writeResponse(w, subscriber, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(GetSubscriberAction, email, getClientIP(r), "User retrieved subscriber: "+imsi)
	})
}

func CreateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		var params CreateSubscriberParams

		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.PolicyName == "" {
			writeError(w, http.StatusBadRequest, "Missing policyName parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.SequenceNumber == "" {
			writeError(w, http.StatusBadRequest, "Missing sequenceNumber parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if !isImsiValid(r.Context(), params.Imsi, dbInstance) {
			writeError(w, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.", errors.New("validation error"), logger.APILog)
			return
		}

		if !isSequenceNumberValid(params.SequenceNumber) {
			writeError(w, http.StatusBadRequest, "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if !isHexString(params.Key) {
			writeError(w, http.StatusBadRequest, "Invalid key format. Must be a 32-character hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if params.Opc != "" && !isHexString(params.Opc) {
			writeError(w, http.StatusBadRequest, "Invalid OPC format. Must be a 32-character hex string.", errors.New("validation error"), logger.APILog)
			return
		}

		keyBytes, _ := hex.DecodeString(params.Key)
		opcHex := params.Opc
		if opcHex == "" {
			operatorCode, err := dbInstance.GetOperatorCode(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to get operator code", err, logger.APILog)
				return
			}
			opBytes, _ := hex.DecodeString(operatorCode)
			derivedOPC, _ := deriveOPc(keyBytes, opBytes)
			opcHex = hex.EncodeToString(derivedOPC)
		}

		if _, err := dbInstance.GetSubscriber(r.Context(), params.Imsi); err == nil {
			writeError(w, http.StatusBadRequest, "Subscriber already exists", errors.New("duplicate"), logger.APILog)
			return
		}

		policy, err := dbInstance.GetPolicy(r.Context(), params.PolicyName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", err, logger.APILog)
			return
		}

		newSubscriber := &db.Subscriber{
			Imsi:           params.Imsi,
			SequenceNumber: params.SequenceNumber,
			PermanentKey:   params.Key,
			Opc:            opcHex,
			PolicyID:       policy.ID,
		}
		if err := dbInstance.CreateSubscriber(r.Context(), newSubscriber); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create subscriber", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber created successfully"}, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(CreateSubscriberAction, email, getClientIP(r), "User created subscriber: "+params.Imsi)
	})
}

func UpdateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		imsi := pathParam(r.URL.Path, "/api/v1/subscribers/")
		if imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		var params UpdateSubscriberParams

		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.PolicyName == "" {
			writeError(w, http.StatusBadRequest, "Missing policyName parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if !isImsiValid(r.Context(), params.Imsi, dbInstance) {
			writeError(w, http.StatusBadRequest, "Invalid IMSI", errors.New("validation error"), logger.APILog)
			return
		}

		existing, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			writeError(w, http.StatusNotFound, "Subscriber not found", err, logger.APILog)
			return
		}
		policy, err := dbInstance.GetPolicy(r.Context(), params.PolicyName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", err, logger.APILog)
			return
		}

		updated := &db.Subscriber{
			Imsi:           existing.Imsi,
			SequenceNumber: existing.SequenceNumber,
			PermanentKey:   existing.PermanentKey,
			Opc:            existing.Opc,
			PolicyID:       policy.ID,
		}
		if err := dbInstance.UpdateSubscriber(r.Context(), updated); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update subscriber", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateSubscriberAction, email, getClientIP(r), "User updated subscriber: "+imsi)
	})
}

func DeleteSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		imsi := pathParam(r.URL.Path, "/api/v1/subscribers/")
		if imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}
		if _, err := dbInstance.GetSubscriber(r.Context(), imsi); err != nil {
			writeError(w, http.StatusNotFound, "Subscriber not found", err, logger.APILog)
			return
		}
		if err := dbInstance.DeleteSubscriber(r.Context(), imsi); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to delete subscriber", err, logger.APILog)
			return
		}
		writeResponse(w, SuccessResponse{Message: "Subscriber deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeleteSubscriberAction, email, getClientIP(r), "User deleted subscriber: "+imsi)
	})
}
