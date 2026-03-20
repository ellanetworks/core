package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/etsi"
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

// SubscriberStatus is the lightweight status returned by the list endpoint.
// It preserves the same fields as the main branch for backward compatibility.
type SubscriberStatus struct {
	Registered bool   `json:"registered"`
	IPAddress  string `json:"ipAddress"`
	LastSeenAt string `json:"lastSeenAt,omitempty"`
}

// Subscriber is the summary representation returned by the list endpoint.
type Subscriber struct {
	Imsi       string           `json:"imsi"`
	PolicyName string           `json:"policyName"`
	Radio      string           `json:"radio,omitempty"`
	Status     SubscriberStatus `json:"status"`
}

type ListSubscribersResponse struct {
	Items      []Subscriber `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

// SubscriberDetailStatus is the rich status returned by the get-single endpoint.
type SubscriberDetailStatus struct {
	Registered         bool   `json:"registered"`
	IPAddress          string `json:"ipAddress"`
	Imei               string `json:"imei"`
	CipheringAlgorithm string `json:"cipheringAlgorithm"`
	IntegrityAlgorithm string `json:"integrityAlgorithm"`
	LastSeenAt         string `json:"lastSeenAt,omitempty"`
	LastSeenRadio      string `json:"lastSeenRadio,omitempty"`
}

// SubscriberDetail is the full representation returned by the get-single endpoint.
type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	PolicyName  string                 `json:"policyName"`
	Status      SubscriberDetailStatus `json:"status"`
	PDUSessions []SessionInfo          `json:"pdu_sessions"`
}

// SubscriberCredentials is the response for the dedicated credentials endpoint.
type SubscriberCredentials struct {
	Key            string `json:"key"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

// SessionInfo is a minimal representation of a PDU session returned by the API.
type SessionInfo struct {
	Status    string `json:"status"`
	IPAddress string `json:"ipAddress,omitempty"`
}

const (
	CreateSubscriberAction = "create_subscriber"
	UpdateSubscriberAction = "update_subscriber"
	DeleteSubscriberAction = "delete_subscriber"
)

const (
	MaxNumSubscribers = 1000
	MaxPDUSessions    = 16
)

func isImsiValid(ctx context.Context, imsi string, dbInstance *db.Database) bool {
	if _, err := etsi.NewSUPIFromIMSI(imsi); err != nil {
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
		radioFilter := q.Get("radio")

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		// When a radio filter is set, we need to fetch all subscribers and
		// filter by the runtime AMF state, then paginate in memory.
		var radioIMSIs map[string]struct{}

		if radioFilter != "" {
			amf := amfContext.AMFSelf()

			// Verify the radio exists.
			_, ranList := amf.ListAmfRan(1, 1000)

			found := false

			for _, radio := range ranList {
				if radio.Name == radioFilter {
					found = true

					break
				}
			}

			if !found {
				writeError(r.Context(), w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio %q not found", radioFilter), logger.APILog)
				return
			}

			// Use the authoritative registration state to find subscribers
			// connected through this radio.
			imsis := amf.RegisteredSubscribersForRadio(radioFilter)
			radioIMSIs = make(map[string]struct{}, len(imsis))

			for _, imsi := range imsis {
				radioIMSIs[imsi] = struct{}{}
			}
		}

		// When filtering by radio we must load all subscribers and paginate
		// in memory because the filter is against runtime AMF state, not the DB.
		dbPage := page
		dbPerPage := perPage

		if radioIMSIs != nil {
			dbPage = 1
			dbPerPage = MaxNumSubscribers
		}

		dbSubscribers, total, err := dbInstance.ListSubscribersPage(ctx, dbPage, dbPerPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list subscribers", err, logger.APILog)
			return
		}

		items := make([]Subscriber, 0, len(dbSubscribers))

		// Pre-fetch all policies into a lookup map.
		// These are small reference tables, so loading them all avoids
		// N+1 queries per subscriber in the loop below.
		allPolicies, _, err := dbInstance.ListPoliciesPage(ctx, 1, 1000)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list policies", err, logger.APILog)
			return
		}

		policyByID := make(map[int]*db.Policy, len(allPolicies))
		for i := range allPolicies {
			policyByID[allPolicies[i].ID] = &allPolicies[i]
		}

		for _, dbSubscriber := range dbSubscribers {
			if radioIMSIs != nil {
				if _, ok := radioIMSIs[dbSubscriber.Imsi]; !ok {
					continue
				}
			}

			policy, ok := policyByID[dbSubscriber.PolicyID]
			if !ok {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", fmt.Errorf("policy ID %d not found", dbSubscriber.PolicyID), logger.APILog)
				return
			}

			ipAddress := ""
			if dbSubscriber.IPAddress != nil {
				ipAddress = *dbSubscriber.IPAddress
			}

			amf := amfContext.AMFSelf()

			supi, err := etsi.NewSUPIFromIMSI(dbSubscriber.Imsi)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Invalid subscriber IMSI", err, logger.APILog)
				return
			}

			subscriberStatus := SubscriberStatus{
				Registered: amf.IsSubscriberRegistered(supi),
				IPAddress:  ipAddress,
			}

			if lastSeen := amf.LastSeenAtForSubscriber(supi); !lastSeen.IsZero() {
				subscriberStatus.LastSeenAt = lastSeen.UTC().Format(time.RFC3339)
			}

			items = append(items, Subscriber{
				Imsi:       dbSubscriber.Imsi,
				PolicyName: policy.Name,
				Radio:      amf.RadioNameForSubscriber(supi),
				Status:     subscriberStatus,
			})
		}

		// When filtering by radio, apply pagination in memory.
		if radioIMSIs != nil {
			total = len(items)
			start := (page - 1) * perPage
			end := start + perPage

			if start > len(items) {
				start = len(items)
			}

			if end > len(items) {
				end = len(items)
			}

			items = items[start:end]
		}

		subscribers := ListSubscribersResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(r.Context(), w, subscribers, http.StatusOK, logger.APILog)
	})
}

func GetSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imsi := r.PathValue("imsi")
		if imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		supi, err := etsi.NewSUPIFromIMSI(imsi)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid IMSI format", err, logger.APILog)
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber", err, logger.APILog)

			return
		}

		policy, err := dbInstance.GetPolicyByID(r.Context(), dbSubscriber.PolicyID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		ipAddress := ""
		if dbSubscriber.IPAddress != nil {
			ipAddress = *dbSubscriber.IPAddress
		}

		amf := amfContext.AMFSelf()

		snap, found := amf.GetUESnapshot(supi)

		subscriberStatus := SubscriberDetailStatus{
			Registered: false,
			IPAddress:  ipAddress,
		}

		if found {
			subscriberStatus.Registered = snap.State == amfContext.Registered
			subscriberStatus.CipheringAlgorithm = snap.CipheringAlgorithm
			subscriberStatus.IntegrityAlgorithm = snap.IntegrityAlgorithm
			subscriberStatus.LastSeenRadio = snap.LastSeenRadio

			if snap.Pei != "" {
				if converted, err := etsi.IMEIFromPEI(snap.Pei); err == nil {
					subscriberStatus.Imei = converted
				} else {
					logger.APILog.Warn("failed to convert PEI to IMEI", logger.PEI(snap.Pei), zap.Error(err))
				}
			}

			if !snap.LastSeenAt.IsZero() {
				subscriberStatus.LastSeenAt = snap.LastSeenAt.UTC().Format(time.RFC3339)
			}
		}

		pduSessions, _ := amfContext.GetUEPDUSessions(supi)

		sessions := make([]SessionInfo, 0, len(pduSessions))
		for _, pdu := range pduSessions {
			session := toSessionInfo(pdu)
			sessions = append(sessions, session)
		}

		if len(sessions) > MaxPDUSessions {
			sessions = sessions[:MaxPDUSessions]
		}

		// Temporary: copy subscriber IP into the first session's IPAddress field so
		// the UI can show it alongside session information. IP ownership should be
		// moved to sessions in a future refactor.
		if ipAddress != "" && len(sessions) > 0 {
			sessions[0].IPAddress = ipAddress
		}

		subscriber := SubscriberDetail{
			Imsi:        dbSubscriber.Imsi,
			PolicyName:  policy.Name,
			Status:      subscriberStatus,
			PDUSessions: sessions,
		}

		writeResponse(r.Context(), w, subscriber, http.StatusOK, logger.APILog)
	})
}

const (
	ViewSubscriberCredentialsAction = "view_subscriber_credentials"
)

func GetSubscriberCredentials(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
		if imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber", err, logger.APILog)

			return
		}

		creds := SubscriberCredentials{
			Key:            dbSubscriber.PermanentKey,
			Opc:            dbSubscriber.Opc,
			SequenceNumber: dbSubscriber.SequenceNumber,
		}

		writeResponse(r.Context(), w, creds, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), ViewSubscriberCredentialsAction, email, getClientIP(r), "User viewed credentials for subscriber: "+imsi)
	})
}

func CreateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateSubscriberParams

		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing imsi parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.PolicyName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing policyName parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.SequenceNumber == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing sequenceNumber parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if !isImsiValid(r.Context(), params.Imsi, dbInstance) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.", errors.New("validation error"), logger.APILog)
			return
		}

		if !isSequenceNumberValid(params.SequenceNumber) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if !isHexString(params.Key) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid key format. Must be a 32-character hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if params.Opc != "" && !isHexString(params.Opc) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid OPC format. Must be a 32-character hex string.", errors.New("validation error"), logger.APILog)
			return
		}

		keyBytes, _ := hex.DecodeString(params.Key)

		opcHex := params.Opc
		if opcHex == "" {
			operatorCode, err := dbInstance.GetOperatorCode(r.Context())
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get operator code", err, logger.APILog)
				return
			}

			opBytes, _ := hex.DecodeString(operatorCode)
			derivedOPC, _ := deriveOPc(keyBytes, opBytes)
			opcHex = hex.EncodeToString(derivedOPC)
		}

		policy, err := dbInstance.GetPolicy(r.Context(), params.PolicyName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		numSubscribers, err := dbInstance.CountSubscribers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count subscribers", err, logger.APILog)
			return
		}

		if numSubscribers >= MaxNumSubscribers {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of subscribers reached ("+strconv.Itoa(MaxNumSubscribers)+")", nil, logger.APILog)
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
				writeError(r.Context(), w, http.StatusConflict, "Subscriber already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create subscriber", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Subscriber created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateSubscriberAction, email, getClientIP(r), "User created subscriber: "+params.Imsi)
	})
}

func UpdateSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
		if imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		var params UpdateSubscriberParams

		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing imsi parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if params.PolicyName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing policyName parameter", errors.New("validation error"), logger.APILog)
			return
		}

		if !isImsiValid(r.Context(), params.Imsi, dbInstance) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid IMSI", errors.New("validation error"), logger.APILog)
			return
		}

		policy, err := dbInstance.GetPolicy(r.Context(), params.PolicyName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		updated := &db.Subscriber{
			Imsi:     params.Imsi,
			PolicyID: policy.ID,
		}
		if err := dbInstance.UpdateSubscriberPolicy(r.Context(), updated); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update subscriber", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Subscriber updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), UpdateSubscriberAction, email, getClientIP(r), "User updated subscriber: "+imsi)
	})
}

func DeleteSubscriber(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
		if imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing imsi parameter", errors.New("imsi required"), logger.APILog)
			return
		}

		if _, err := dbInstance.GetSubscriber(r.Context(), imsi); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber", err, logger.APILog)

			return
		}

		amf := amfContext.AMFSelf()

		supi, err := etsi.NewSUPIFromIMSI(imsi)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Invalid subscriber IMSI", err, logger.APILog)
			return
		}

		err = deregister.DeregisterSubscriber(r.Context(), amf, supi)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to deregister subscriber", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteSubscriber(r.Context(), imsi); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete subscriber", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Subscriber deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSubscriberAction, email, getClientIP(r), "User deleted subscriber: "+imsi)
	})
}

func toSessionInfo(pdu amfContext.PDUSessionExport) SessionInfo {
	status := "active"
	if pdu.Inactive {
		status = "inactive"
	}

	return SessionInfo{
		Status: status,
	}
}
