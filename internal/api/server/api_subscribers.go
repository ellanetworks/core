// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	"github.com/ellanetworks/core/internal/mme"
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

type SubscriberStatus struct {
	Registered       bool     `json:"registered"`
	RadioAccessTypes []string `json:"radio_access_types,omitempty"`
	NumSessions      int      `json:"num_sessions"`
	LastSeenAt       string   `json:"last_seen_at,omitempty"`
}

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

type SubscriberDetailStatus struct {
	Registered         bool     `json:"registered"`
	RadioAccessTypes   []string `json:"radio_access_types,omitempty"`
	Imei               string   `json:"imei"`
	CipheringAlgorithm string   `json:"ciphering_algorithm"`
	IntegrityAlgorithm string   `json:"integrity_algorithm"`
	LastSeenAt         string   `json:"last_seen_at,omitempty"`
	LastSeenRadio      string   `json:"last_seen_radio,omitempty"`
}

type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	ProfileName string                 `json:"profile_name"`
	Status      SubscriberDetailStatus `json:"status"`
	Sessions    []Session              `json:"sessions"`
}

type SubscriberCredentials struct {
	Key            string `json:"key"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

type SNSSAI struct {
	SST int32  `json:"sst"`
	SD  string `json:"sd,omitempty"`
}

type Session struct {
	RadioAccessType string  `json:"radio_access_type"` // "4G" | "5G"
	ID              uint8   `json:"id"`                // PDU Session ID (5G) / linked EPS Bearer ID (4G)
	Status          string  `json:"status"`
	IPType          string  `json:"ip_type,omitempty"` // IPv4 | IPv6 | IPv4v6
	IPv4Address     string  `json:"ipv4_address,omitempty"`
	IPv6Prefix      string  `json:"ipv6_prefix,omitempty"`
	DataNetwork     string  `json:"data_network,omitempty"` // DNN (5G) / APN (4G)
	Slice           *SNSSAI `json:"slice,omitempty"`        // 5G only
	AMBRUplink      string  `json:"ambr_uplink,omitempty"`
	AMBRDownlink    string  `json:"ambr_downlink,omitempty"`
}

const (
	CreateSubscriberAction = "create_subscriber"
	UpdateSubscriberAction = "update_subscriber"
	DeleteSubscriberAction = "delete_subscriber"
)

const (
	MaxNumSubscribers = 1000
	MaxSessions       = 26
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

	return len(imsi) > 3+mncLength
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

func radioIsKnown(amfInstance *amf.AMF, mmeInstance *mme.MME, name string) bool {
	return amfInstance.HasRadio(name) || (mmeInstance != nil && mmeInstance.HasRadio(name))
}

// accessView is what one access — 4G or 5G — knows about a subscriber.
type accessView struct {
	rat        string
	present    bool
	radioName  string
	imei       string
	ciphering  string
	integrity  string
	lastSeenAt time.Time
}

func (v accessView) newerThan(other accessView) bool {
	return !other.present || v.lastSeenAt.After(other.lastSeenAt)
}

type mergedAccess struct {
	RATs       []string
	RadioName  string
	Imei       string
	Ciphering  string
	Integrity  string
	LastSeenAt time.Time
}

func mergeAccesses(views ...accessView) mergedAccess {
	var (
		merged  mergedAccess
		serving accessView
	)

	for _, v := range views {
		if !v.present {
			continue
		}

		merged.RATs = append(merged.RATs, v.rat)

		if merged.Imei == "" {
			merged.Imei = v.imei
		}

		if v.newerThan(serving) {
			serving = v
		}
	}

	merged.RadioName = serving.radioName
	merged.Ciphering, merged.Integrity = serving.ciphering, serving.integrity
	merged.LastSeenAt = serving.lastSeenAt

	return merged
}

func ListSubscribers(dbInstance *db.Database, amfInstance *amf.AMF, mmeInstance *mme.MME) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)
		radioFilter := q.Get("radio")
		dataNetworkFilter := q.Get("data_network")

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		var mmeStatus map[string]mme.ConnectedSubscriber

		if mmeInstance != nil {
			mmeStatus = mmeInstance.ConnectedSubscribers()
		}

		amf5GStatus := amfInstance.ConnectedSubscribers()

		var radioIMSIs map[string]struct{}

		if radioFilter != "" {
			found := radioIsKnown(amfInstance, mmeInstance, radioFilter)
			if !found {
				writeError(r.Context(), w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio %q not found", radioFilter), logger.APILog)
				return
			}

			radioIMSIs = make(map[string]struct{})

			for imsi, cs := range amf5GStatus {
				if cs.RadioName == radioFilter {
					radioIMSIs[imsi] = struct{}{}
				}
			}

			for imsi, st := range mmeStatus {
				if st.RadioName == radioFilter {
					radioIMSIs[imsi] = struct{}{}
				}
			}
		}

		dbPage := page
		dbPerPage := perPage

		if radioIMSIs != nil {
			dbPage = 1
			dbPerPage = MaxNumSubscribers
		}

		var dataNetworkID string

		if dataNetworkFilter != "" {
			dn, dnErr := dbInstance.GetDataNetwork(ctx, dataNetworkFilter)
			if dnErr != nil {
				writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
				return
			}

			dataNetworkID = dn.ID
		}

		var (
			dbSubscribers []db.Subscriber
			total         int
			err           error
		)

		if dataNetworkID != "" {
			dbSubscribers, total, err = dbInstance.ListSubscribersByDataNetworkPage(ctx, dataNetworkID, dbPage, dbPerPage)
		} else {
			dbSubscribers, total, err = dbInstance.ListSubscribersPage(ctx, dbPage, dbPerPage)
		}

		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list subscribers", err, logger.APILog)
			return
		}

		items := make([]Subscriber, 0, len(dbSubscribers))

		allProfiles, _, err := dbInstance.ListProfilesPage(ctx, 1, 1000)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list profiles", err, logger.APILog)
			return
		}

		profileByID := make(map[string]*db.Profile, len(allProfiles))
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
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", fmt.Errorf("no profile for ID %s", dbSubscriber.ProfileID), logger.APILog)
				return
			}

			if _, err := etsi.NewSUPIFromIMSI(dbSubscriber.Imsi); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Invalid subscriber IMSI", err, logger.APILog)
				return
			}

			amf5G, on5G := amf5GStatus[dbSubscriber.Imsi]
			mme4G, on4G := mmeStatus[dbSubscriber.Imsi]

			merged := mergeAccesses(
				accessView{rat: "4G", present: on4G, radioName: mme4G.RadioName, lastSeenAt: mme4G.LastSeenAt},
				accessView{rat: "5G", present: on5G, radioName: amf5G.RadioName, lastSeenAt: amf5G.LastSeenAt},
			)

			subscriberStatus := SubscriberStatus{
				Registered:       on5G || on4G,
				RadioAccessTypes: merged.RATs,
				NumSessions:      mme4G.NumSessions + amf5G.NumSessions,
			}

			if !merged.LastSeenAt.IsZero() {
				subscriberStatus.LastSeenAt = merged.LastSeenAt.UTC().Format(time.RFC3339)
			}

			items = append(items, Subscriber{
				Imsi:        dbSubscriber.Imsi,
				ProfileName: profile.Name,
				Radio:       merged.RadioName,
				Status:      subscriberStatus,
			})
		}

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

func GetSubscriber(dbInstance *db.Database, amfInstance *amf.AMF, mmeInstance *mme.MME) http.Handler {
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

		profile, err := dbInstance.GetProfileByID(r.Context(), dbSubscriber.ProfileID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve profile", err, logger.APILog)
			return
		}

		snap, radioName, pduSessions, found := amfInstance.LookupSubscriber(supi)

		var (
			cs   mme.ConnectedSubscriber
			on4G bool
		)

		if mmeInstance != nil {
			cs, on4G = mmeInstance.LookupSubscriber(imsi)
		}

		merged := mergeAccesses(
			accessView{
				rat: "4G", present: on4G, radioName: cs.RadioName, imei: cs.Imei,
				ciphering: cs.CipheringAlgorithm, integrity: cs.IntegrityAlgorithm, lastSeenAt: cs.LastSeenAt,
			},
			accessView{
				rat: "5G", present: found, radioName: radioName, imei: snap.Imei,
				ciphering: snap.CipheringAlgorithm, integrity: snap.IntegrityAlgorithm, lastSeenAt: snap.LastSeenAt,
			},
		)

		subscriberStatus := SubscriberDetailStatus{
			Registered:         on4G || found,
			RadioAccessTypes:   merged.RATs,
			LastSeenRadio:      merged.RadioName,
			Imei:               merged.Imei,
			CipheringAlgorithm: merged.Ciphering,
			IntegrityAlgorithm: merged.Integrity,
		}

		if !merged.LastSeenAt.IsZero() {
			subscriberStatus.LastSeenAt = merged.LastSeenAt.UTC().Format(time.RFC3339)
		}

		sessions := make([]Session, 0, len(pduSessions)+len(cs.Sessions))

		for i := range cs.Sessions {
			if len(sessions) >= MaxSessions {
				break
			}

			sessions = append(sessions, sessionFrom4G(&cs.Sessions[i]))
		}

		if found {
			for _, pdu := range pduSessions {
				if len(sessions) >= MaxSessions {
					break
				}

				sessions = append(sessions, sessionFrom5G(pdu))
			}
		}

		subscriber := SubscriberDetail{
			Imsi:        dbSubscriber.Imsi,
			ProfileName: profile.Name,
			Status:      subscriberStatus,
			Sessions:    sessions,
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
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid IMSI format. Must be a string of 6 to 15 digits starting with `<mcc><mnc>`.", errors.New("validation error"), logger.APILog)
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

func DeleteSubscriber(dbInstance *db.Database, amfInstance *amf.AMF, mmeInstance *mme.MME) http.Handler {
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

		if mmeInstance != nil {
			mmeInstance.DetachSubscriber(r.Context(), imsi)
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

func sessionFrom4G(s *mme.SubscriberSession) Session {
	return Session{
		RadioAccessType: "4G",
		ID:              s.BearerID,
		Status:          "active",
		IPType:          ipTypeName(uint8(s.PDNType)),
		IPv4Address:     s.IPv4Address,
		IPv6Prefix:      s.IPv6Prefix,
		DataNetwork:     s.APN,
		AMBRUplink:      s.AMBRUplink,
		AMBRDownlink:    s.AMBRDownlink,
	}
}

func sessionFrom5G(pdu amf.PDUSessionExport) Session {
	status := "active"
	if pdu.Inactive {
		status = "inactive"
	}

	s := Session{
		RadioAccessType: "5G",
		ID:              pdu.PDUSessionID,
		Status:          status,
		IPType:          ipTypeName(pdu.PDUSessionType),
		IPv4Address:     pdu.PDUIPV4Address,
		IPv6Prefix:      pdu.PDUIPV6Prefix,
		DataNetwork:     pdu.DNN,
	}
	if pdu.Snssai != nil {
		s.Slice = &SNSSAI{SST: pdu.Snssai.Sst, SD: pdu.Snssai.Sd}
	}

	if pdu.PolicyData != nil && pdu.PolicyData.Ambr != nil {
		s.AMBRUplink = pdu.PolicyData.Ambr.Uplink.String()
		s.AMBRDownlink = pdu.PolicyData.Ambr.Downlink.String()
	}

	return s
}

func ipTypeName(t uint8) string {
	switch t {
	case 1:
		return "IPv4"
	case 2:
		return "IPv6"
	case 3:
		return "IPv4v6"
	default:
		return ""
	}
}
