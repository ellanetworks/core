package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/deregister"
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

type SubscriberStatus struct {
	Registered bool   `json:"registered"`
	IPAddress  string `json:"ipAddress"`
}

type Subscriber struct {
	Imsi           string           `json:"imsi"`
	Opc            string           `json:"opc"`
	SequenceNumber string           `json:"sequenceNumber"`
	Key            string           `json:"key"`
	PolicyName     string           `json:"policyName"`
	Status         SubscriberStatus `json:"status"`
}

type ListSubscribersResponse struct {
	Items      []Subscriber `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

const (
	CreateSubscriberAction = "create_subscriber"
	UpdateSubscriberAction = "update_subscriber"
	DeleteSubscriberAction = "delete_subscriber"
)

const (
	MaxNumSubscribers = 1000
)

func isImsiValid(ctx context.Context, imsi string, dbInstance *db.Database) bool {
	if !isImsiValidRegexp(imsi) {
		return false
	}

	network, err := dbInstance.GetOperator(ctx)
	if err != nil {
		logger.APILog.Warn("Failed to retrieve operator", zap.Error(err))
		return false
	}

	Mcc := network.Mcc
	Mnc := network.Mnc

	mncLength := len(Mnc)

	if imsi[:3] != Mcc || imsi[3:3+mncLength] != Mnc {
		return false
	}

	return true
}

func isImsiValidRegexp(imsi string) bool {
	if len(imsi) != 15 {
		return false
	}

	for _, c := range imsi {
		if c < '0' || c > '9' {
			return false
		}
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
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		dbSubscribers, total, err := dbInstance.ListSubscribersPage(ctx, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list subscribers", err, logger.APILog)
			return
		}

		items := make([]Subscriber, 0, len(dbSubscribers))

		for _, dbSubscriber := range dbSubscribers {
			policy, err := dbInstance.GetPolicyByID(ctx, dbSubscriber.PolicyID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}

			ipAddress := ""
			if dbSubscriber.IPAddress != nil {
				ipAddress = *dbSubscriber.IPAddress
			}

			amf := amfContext.AMFSelf()

			subscriberStatus := SubscriberStatus{
				Registered: amf.IsSubscriberRegistered(dbSubscriber.Imsi),
				IPAddress:  ipAddress,
			}

			items = append(items, Subscriber{
				Imsi:           dbSubscriber.Imsi,
				Opc:            dbSubscriber.Opc,
				Key:            dbSubscriber.PermanentKey,
				SequenceNumber: dbSubscriber.SequenceNumber,
				PolicyName:     policy.Name,
				Status:         subscriberStatus,
			})
		}

		subscribers := ListSubscribersResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, subscribers, http.StatusOK, logger.APILog)
	})
}

func GetSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imsi := r.PathValue("imsi")
		if imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber", err, logger.APILog)

			return
		}

		policy, err := dbInstance.GetPolicyByID(r.Context(), dbSubscriber.PolicyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		ipAddress := ""
		if dbSubscriber.IPAddress != nil {
			ipAddress = *dbSubscriber.IPAddress
		}

		amf := amfContext.AMFSelf()

		subscriberStatus := SubscriberStatus{
			Registered: amf.IsSubscriberRegistered(dbSubscriber.Imsi),
			IPAddress:  ipAddress,
		}

		subscriber := Subscriber{
			Imsi:           dbSubscriber.Imsi,
			Opc:            dbSubscriber.Opc,
			SequenceNumber: dbSubscriber.SequenceNumber,
			Key:            dbSubscriber.PermanentKey,
			PolicyName:     policy.Name,
			Status:         subscriberStatus,
		}

		writeResponse(w, subscriber, http.StatusOK, logger.APILog)
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

		policy, err := dbInstance.GetPolicy(r.Context(), params.PolicyName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		numSubscribers, err := dbInstance.CountSubscribers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count subscribers", err, logger.APILog)
			return
		}

		if numSubscribers >= MaxNumSubscribers {
			writeError(w, http.StatusBadRequest, "Maximum number of subscribers reached ("+strconv.Itoa(MaxNumSubscribers)+")", nil, logger.APILog)
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
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(w, http.StatusConflict, "Subscriber already exists", nil, logger.APILog)
				return
			}

			writeError(w, http.StatusInternalServerError, "Failed to create subscriber", err, logger.APILog)

			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateSubscriberAction, email, getClientIP(r), "User created subscriber: "+params.Imsi)
	})
}

func UpdateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
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

		policy, err := dbInstance.GetPolicy(r.Context(), params.PolicyName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		updated := &db.Subscriber{
			Imsi:     params.Imsi,
			PolicyID: policy.ID,
		}
		if err := dbInstance.UpdateSubscriberPolicy(r.Context(), updated); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(w, http.StatusInternalServerError, "Failed to update subscriber", err, logger.APILog)

			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), UpdateSubscriberAction, email, getClientIP(r), "User updated subscriber: "+imsi)
	})
}

func DeleteSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
		if imsi == "" {
			writeError(w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetSubscriber(r.Context(), imsi); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber", err, logger.APILog)

			return
		}

		amf := amfContext.AMFSelf()

		err := deregister.DeregisterSubscriber(r.Context(), amf, imsi)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to deregister subscriber", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteSubscriber(r.Context(), imsi); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(w, http.StatusInternalServerError, "Failed to delete subscriber", err, logger.APILog)

			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSubscriberAction, email, getClientIP(r), "User deleted subscriber: "+imsi)
	})
}
