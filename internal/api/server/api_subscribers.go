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
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type CreateSubscriberParams struct {
	Imsi           string `json:"imsi"`
	Key            string `json:"key"`
	Opc            string `json:"opc,omitempty"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profile_name"`
}

type UpdateSubscriberParams struct {
	ProfileName string `json:"profile_name"`
}

// SubscriberStatus is the lightweight status returned by the list endpoint.
type SubscriberStatus struct {
	Registered     bool   `json:"registered"`
	NumPDUSessions int    `json:"num_pdu_sessions"`
	LastSeenAt     string `json:"lastSeenAt,omitempty"`
}

// Subscriber is the summary representation returned by the list endpoint.
type Subscriber struct {
	Imsi        string           `json:"imsi"`
	ProfileName string           `json:"profile_name"`
	Radio       string           `json:"radio,omitempty"`
	Status      SubscriberStatus `json:"status"`
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
	Imei               string `json:"imei"`
	CipheringAlgorithm string `json:"cipheringAlgorithm"`
	IntegrityAlgorithm string `json:"integrityAlgorithm"`
	LastSeenAt         string `json:"lastSeenAt,omitempty"`
	LastSeenRadio      string `json:"lastSeenRadio,omitempty"`
}

// SubscriberDetail is the full representation returned by the get-single endpoint.
type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	ProfileName string                 `json:"profile_name"`
	Status      SubscriberDetailStatus `json:"status"`
	PDUSessions []SessionInfo          `json:"pdu_sessions"`
}

// SubscriberCredentials is the response for the dedicated credentials endpoint.
type SubscriberCredentials struct {
	Key            string `json:"key"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

// SessionInfo is a representation of a PDU session returned by the API.
type SessionInfo struct {
	PDUSessionID    uint8  `json:"pdu_session_id"`
	Status          string `json:"status"`
	IPAddress       string `json:"ipAddress,omitempty"`
	DNN             string `json:"dnn,omitempty"`
	SST             int32  `json:"sst,omitempty"`
	SD              string `json:"sd,omitempty"`
	SessionAmbrUp   string `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDown string `json:"session_ambr_downlink,omitempty"`
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

func isHexOfLength(input string, byteLength int) bool {
	b, err := hex.DecodeString(input)
	if err != nil {
		return false
	}

	return len(b) == byteLength
}

func isSequenceNumberValid(sequenceNumber string) bool {
	bytes, err := hex.DecodeString(sequenceNumber)
	if err != nil {
		return false
	}

	return len(bytes) == 6
}

func ListSubscribers(dbInstance *db.Database, amfInstance *amf.AMF) http.Handler {
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
			// Verify the radio exists.
			_, ranList := amfInstance.ListAmfRan(1, 1000)

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
			imsis := amfInstance.RegisteredSubscribersForRadio(radioFilter)
			radioIMSIs = make(map[string]struct{}, len(imsis))

			for _, imsi := range imsis {
				radioIMSIs[imsi] = struct{}{}
			}
		}

		// When filtering by radio we must load all subscribers and paginate
		// in memory because the filter is against runtime AMF state, not the DB.
		// Future improvement: if the subscriber count grows large, push this
		// filter into a DB-side join or maintain a radio→subscriber mapping.
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

		// Pre-fetch all profiles into a lookup map keyed by ID.
		allProfiles, _, err := dbInstance.ListProfilesPage(ctx, 1, 1000)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list profiles", err, logger.APILog)
			return
		}

		profileByID := make(map[int]*db.Profile, len(allProfiles))
		for i := range allProfiles {
			profileByID[allProfiles[i].ID] = &allProfiles[i]
		}

		for _, dbSubscriber := range dbSubscribers {
			if radioIMSIs != nil {
				if _, ok := radioIMSIs[dbSubscriber.Imsi]; !ok {
					continue
				}
			}

			profile, ok := profileByID[dbSubscriber.ProfileID]
			if !ok {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", fmt.Errorf("no profile for ID %d", dbSubscriber.ProfileID), logger.APILog)
				return
			}

			supi, err := etsi.NewSUPIFromIMSI(dbSubscriber.Imsi)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Invalid subscriber IMSI", err, logger.APILog)
				return
			}

			subscriberStatus := SubscriberStatus{
				Registered:     amfInstance.IsSubscriberRegistered(supi),
				NumPDUSessions: amfInstance.CountUEPDUSessions(supi),
			}

			if lastSeen := amfInstance.LastSeenAtForSubscriber(supi); !lastSeen.IsZero() {
				subscriberStatus.LastSeenAt = lastSeen.UTC().Format(time.RFC3339)
			}

			items = append(items, Subscriber{
				Imsi:        dbSubscriber.Imsi,
				ProfileName: profile.Name,
				Radio:       amfInstance.RadioNameForSubscriber(supi),
				Status:      subscriberStatus,
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

func GetSubscriber(dbInstance *db.Database, amfInstance *amf.AMF) http.Handler {
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

		// Find the profile for this subscriber.
		profile, err := dbInstance.GetProfileByID(r.Context(), dbSubscriber.ProfileID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)
			return
		}

		snap, found := amfInstance.GetUESnapshot(supi)

		subscriberStatus := SubscriberDetailStatus{
			Registered: false,
		}

		if found {
			subscriberStatus.Registered = snap.State == amf.Registered
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

		pduSessions, _ := amfInstance.GetUEPDUSessions(supi)

		sessions := make([]SessionInfo, 0, len(pduSessions))
		for _, pdu := range pduSessions {
			session := toSessionInfo(pdu)
			sessions = append(sessions, session)
		}

		if len(sessions) > MaxPDUSessions {
			sessions = sessions[:MaxPDUSessions]
		}

		subscriber := SubscriberDetail{
			Imsi:        dbSubscriber.Imsi,
			ProfileName: profile.Name,
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

		if params.ProfileName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing profile_name parameter", errors.New("validation error"), logger.APILog)
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

		if !isHexOfLength(params.Key, 16) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid key format. Must be a 32-character hexadecimal string.", errors.New("validation error"), logger.APILog)
			return
		}

		if params.Opc != "" && !isHexOfLength(params.Opc, 16) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid OPC format. Must be a 32-character hexadecimal string.", errors.New("validation error"), logger.APILog)
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

		profile, err := dbInstance.GetProfile(r.Context(), params.ProfileName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
			return
		}

		policyCount, err := dbInstance.CountPoliciesInProfile(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check policies", err, logger.APILog)
			return
		}

		if policyCount < 1 {
			writeError(r.Context(), w, http.StatusConflict, "Profile has no policy; create a policy for this profile before assigning subscribers", nil, logger.APILog)
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
			ProfileID:      profile.ID,
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

		if params.ProfileName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing profile_name parameter", errors.New("validation error"), logger.APILog)
			return
		}

		profile, err := dbInstance.GetProfile(r.Context(), params.ProfileName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
			return
		}

		policyCount, err := dbInstance.CountPoliciesInProfile(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check policies", err, logger.APILog)
			return
		}

		if policyCount < 1 {
			writeError(r.Context(), w, http.StatusConflict, "Profile has no policy; create a policy for this profile before assigning subscribers", nil, logger.APILog)
			return
		}

		updated := &db.Subscriber{
			Imsi:      imsi,
			ProfileID: profile.ID,
		}
		if err := dbInstance.UpdateSubscriberProfile(r.Context(), updated); err != nil {
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

func DeleteSubscriber(dbInstance *db.Database, amfInstance *amf.AMF) http.Handler {
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

		supi, err := etsi.NewSUPIFromIMSI(imsi)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Invalid subscriber IMSI", err, logger.APILog)
			return
		}

		amfInstance.DeregisterSubscriber(r.Context(), supi)

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

func toSessionInfo(pdu amf.PDUSessionExport) SessionInfo {
	status := "active"
	if pdu.Inactive {
		status = "inactive"
	}

	s := SessionInfo{
		PDUSessionID: pdu.PDUSessionID,
		Status:       status,
		IPAddress:    pdu.PDUAddress,
		DNN:          pdu.DNN,
	}
	if pdu.Snssai != nil {
		s.SST = pdu.Snssai.Sst
		s.SD = pdu.Snssai.Sd
	}

	if pdu.PolicyData != nil && pdu.PolicyData.Ambr != nil {
		s.SessionAmbrUp = pdu.PolicyData.Ambr.Uplink
		s.SessionAmbrDown = pdu.PolicyData.Ambr.Downlink
	}

	return s
}
