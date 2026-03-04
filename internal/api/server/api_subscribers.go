package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

type SubscriberStatus struct {
	Registered         bool   `json:"registered"`
	IPAddress          string `json:"ipAddress"`
	State              string `json:"state"`
	ConnectedRadio     string `json:"connectedRadio"`
	Imei               string `json:"imei"`
	Tac                string `json:"tac"`
	CellID             string `json:"cellID"`
	ActiveSessions     int    `json:"activeSessions"`
	AmbrUplink         string `json:"ambrUplink"`
	AmbrDownlink       string `json:"ambrDownlink"`
	CipheringAlgorithm string `json:"cipheringAlgorithm"`
	IntegrityAlgorithm string `json:"integrityAlgorithm"`
}

type Subscriber struct {
	Imsi            string           `json:"imsi"`
	Opc             string           `json:"opc"`
	SequenceNumber  string           `json:"sequenceNumber"`
	Key             string           `json:"key"`
	PolicyName      string           `json:"policyName"`
	DataNetworkName string           `json:"dataNetworkName"`
	Status          SubscriberStatus `json:"status"`
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

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		dbSubscribers, total, err := dbInstance.ListSubscribersPage(ctx, page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list subscribers", err, logger.APILog)
			return
		}

		items := make([]Subscriber, 0, len(dbSubscribers))

		for _, dbSubscriber := range dbSubscribers {
			policy, err := dbInstance.GetPolicyByID(ctx, dbSubscriber.PolicyID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}

			dataNetwork, err := dbInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve data network", err, logger.APILog)
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

			items = append(items, Subscriber{
				Imsi:            dbSubscriber.Imsi,
				Opc:             dbSubscriber.Opc,
				Key:             dbSubscriber.PermanentKey,
				SequenceNumber:  dbSubscriber.SequenceNumber,
				PolicyName:      policy.Name,
				DataNetworkName: dataNetwork.Name,
				Status:          subscriberStatus,
			})
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

		if _, err := etsi.NewSUPIFromIMSI(imsi); err != nil {
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

		dataNetwork, err := dbInstance.GetDataNetworkByID(r.Context(), policy.DataNetworkID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve data network", err, logger.APILog)
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

		state := "Deregistered"
		connectedRadio := ""
		imei := ""
		tac := ""
		cellID := ""
		activeSessions := 0
		ambrUplink := ""
		ambrDownlink := ""
		cipheringAlgorithm := ""
		integrityAlgorithm := ""

		registered := amf.IsSubscriberRegistered(supi)

		if ue, ok := amf.FindAMFUEBySupi(supi); ok {
			state = string(ue.GetState())

			if ue.RanUe != nil && ue.RanUe.Radio != nil {
				connectedRadio = ue.RanUe.Radio.Name
			}

			if ue.Pei != "" {
				if converted, err := etsi.IMEIFromPEI(ue.Pei); err == nil {
					imei = converted
				}
			}

			if ue.Tai.Tac != "" {
				tac = ue.Tai.Tac
			}

			if ue.Location.NrLocation != nil && ue.Location.NrLocation.Ncgi != nil {
				cellID = ue.Location.NrLocation.Ncgi.NrCellID
			}

			for _, sm := range ue.SmContextList {
				if !sm.PduSessionInactive {
					activeSessions++
				}
			}

			if ue.Ambr != nil {
				ambrUplink = ue.Ambr.Uplink
				ambrDownlink = ue.Ambr.Downlink
			}

			cipheringAlgorithm = ue.CipheringAlgName()
			integrityAlgorithm = ue.IntegrityAlgName()
		}

		subscriberStatus := SubscriberStatus{
			Registered:         registered,
			IPAddress:          ipAddress,
			State:              state,
			ConnectedRadio:     connectedRadio,
			Imei:               imei,
			Tac:                tac,
			CellID:             cellID,
			ActiveSessions:     activeSessions,
			AmbrUplink:         ambrUplink,
			AmbrDownlink:       ambrDownlink,
			CipheringAlgorithm: cipheringAlgorithm,
			IntegrityAlgorithm: integrityAlgorithm,
		}

		subscriber := Subscriber{
			Imsi:            dbSubscriber.Imsi,
			Opc:             dbSubscriber.Opc,
			SequenceNumber:  dbSubscriber.SequenceNumber,
			Key:             dbSubscriber.PermanentKey,
			PolicyName:      policy.Name,
			DataNetworkName: dataNetwork.Name,
			Status:          subscriberStatus,
		}

		writeResponse(r.Context(), w, subscriber, http.StatusOK, logger.APILog)
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
