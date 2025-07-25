package server

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type UpdateOperatorSliceParams struct {
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
}

type UpdateOperatorTrackingParams struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type UpdateOperatorIDParams struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type UpdateOperatorCodeParams struct {
	OperatorCode string `json:"operatorCode,omitempty"`
}

type UpdateOperatorHomeNetworkParams struct {
	PrivateKey string `json:"privateKey,omitempty"`
}

type GetOperatorTrackingResponse struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type GetOperatorResponse struct {
	ID          GetOperatorIDResponse          `json:"id,omitempty"`
	Slice       GetOperatorSliceResponse       `json:"slice,omitempty"`
	Tracking    GetOperatorTrackingResponse    `json:"tracking,omitempty"`
	HomeNetwork GetOperatorHomeNetworkResponse `json:"homeNetwork,omitempty"`
}

type GetOperatorSliceResponse struct {
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
}

type GetOperatorIDResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetOperatorHomeNetworkResponse struct {
	PublicKey string `json:"publicKey,omitempty"`
}

const (
	GetOperatorAction               = "get_operator"
	GetOperatorSliceAction          = "get_operator_slice"
	GetOperatorTrackingAction       = "get_operator_tracking"
	GetOperatorIDAction             = "get_operator_id"
	GetOperatorHomeNetworkAction    = "get_operator_home_network"
	UpdateOperatorSliceAction       = "update_operator_slice"
	UpdateOperatorTrackingAction    = "update_operator_tracking"
	UpdateOperatorIDAction          = "update_operator_id"
	UpdateOperatorCodeAction        = "update_operator_code"
	UpdateOperatorHomeNetworkAction = "update_operator_home_network"
)

// Mcc is a 3-decimal digit
func isValidMcc(mcc string) bool {
	if len(mcc) != 3 {
		return false
	}
	for _, c := range mcc {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Mnc is a 2 or 3-decimal digit
func isValidMnc(mnc string) bool {
	if len(mnc) != 2 && len(mnc) != 3 {
		return false
	}
	for _, c := range mnc {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Operator code is a 32-character hexadecimal string
func isValidOperatorCode(operatorCode string) bool {
	if len(operatorCode) != 32 {
		logger.APILog.Warn("Invalid operator code length", zap.Int("length", len(operatorCode)))
		return false
	}
	_, err := hex.DecodeString(operatorCode)
	if err != nil {
		logger.APILog.Warn("Invalid operator code", zap.Error(err), zap.String("operatorCode", operatorCode))
		return false
	}
	return true
}

// isValidPrivateKey validates whether the provided private key is a valid 32-byte Curve25519 private key.
func isValidPrivateKey(privateKey string) bool {
	// Ensure it is exactly 64 hex characters (32 bytes)
	if len(privateKey) != 64 {
		log.Println("Invalid private key length:", len(privateKey))
		return false
	}

	// Decode from hex string to bytes
	privateKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		log.Println("Invalid private key format:", err)
		return false
	}

	// Ensure it is exactly 32 bytes long
	if len(privateKeyBytes) != 32 {
		log.Println("Invalid private key byte length:", len(privateKeyBytes))
		return false
	}

	// Check if it is correctly clamped for Curve25519 (X25519)
	// - First byte: Bits 0-2 must be cleared
	// - Last byte: Bit 7 must be cleared, and bit 6 must be set
	if privateKeyBytes[0]&7 != 0 || privateKeyBytes[31]&0x80 != 0 || privateKeyBytes[31]&0x40 == 0 {
		log.Println("Invalid Curve25519 key clamping")
		return false
	}

	return true
}

// TAC is a 24-bit identifier
func isValidTac(tac string) bool {
	if len(tac) != 3 {
		return false
	}
	_, err := strconv.ParseInt(tac, 10, 32)
	return err == nil
}

// SST is an 8-bit integer
func isValidSst(sst int) bool {
	return sst >= 0 && sst <= 0xFF
}

// SD is a 24-bit integer
func isValidSd(sd int) bool {
	return sd >= 0 && sd <= 0xFFFFFF
}

func GetOperator(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		hnPublicKey, err := dbOperator.GetHomeNetworkPublicKey()
		if err != nil {
			logger.APILog.Warn("Failed to get home network public key", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to get home network public key", err, logger.APILog)
			return
		}

		operator := &GetOperatorResponse{
			ID: GetOperatorIDResponse{
				Mcc: dbOperator.Mcc,
				Mnc: dbOperator.Mnc,
			},
			Slice: GetOperatorSliceResponse{
				Sst: int(dbOperator.Sst),
				Sd:  dbOperator.Sd,
			},
			Tracking: GetOperatorTrackingResponse{
				SupportedTacs: dbOperator.GetSupportedTacs(),
			},
			HomeNetwork: GetOperatorHomeNetworkResponse{
				PublicKey: hnPublicKey,
			},
		}

		writeResponse(w, operator, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			GetOperatorAction,
			email,
			getClientIP(r),
			"User retrieved operator information",
		)
	})
}

func GetOperatorSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorSlice := &GetOperatorSliceResponse{
			Sst: int(dbOperator.Sst),
			Sd:  dbOperator.Sd,
		}

		writeResponse(w, operatorSlice, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			GetOperatorSliceAction,
			email,
			getClientIP(r),
			"User retrieved operator slice",
		)
	})
}

func GetOperatorTracking(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorTracking := &GetOperatorTrackingResponse{
			SupportedTacs: dbOperator.GetSupportedTacs(),
		}

		writeResponse(w, operatorTracking, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			GetOperatorTrackingAction,
			email,
			getClientIP(r),
			"User retrieved operator tracking information",
		)
	})
}

func GetOperatorID(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorID := &GetOperatorIDResponse{
			Mcc: dbOperator.Mcc,
			Mnc: dbOperator.Mnc,
		}

		writeResponse(w, operatorID, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			GetOperatorIDAction,
			email,
			getClientIP(r),
			"User retrieved operator Id",
		)
	})
}

func UpdateOperatorSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorSliceParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Sst == 0 {
			writeError(w, http.StatusBadRequest, "sst is missing", nil, logger.APILog)
			return
		}
		if params.Sd == 0 {
			writeError(w, http.StatusBadRequest, "sd is missing", nil, logger.APILog)
			return
		}
		if !isValidSst(params.Sst) {
			writeError(w, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer", nil, logger.APILog)
			return
		}
		if !isValidSd(params.Sd) {
			writeError(w, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit integer", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorSlice(r.Context(), int32(params.Sst), params.Sd); err != nil {
			logger.APILog.Warn("Failed to update operator slice information", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to update operator slice information", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator slice information updated successfully"}
		writeResponse(w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			UpdateOperatorSliceAction,
			email,
			getClientIP(r),
			"User updated operator slice information",
		)
	})
}

func UpdateOperatorTracking(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorTrackingParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}
		if len(params.SupportedTacs) == 0 {
			writeError(w, http.StatusBadRequest, "supportedTacs is missing", nil, logger.APILog)
			return
		}

		for _, tac := range params.SupportedTacs {
			if !isValidTac(tac) {
				writeError(w, http.StatusBadRequest, "Invalid TAC format. Must be a 3-digit number", nil, logger.APILog)
				return
			}
		}

		if err := dbInstance.UpdateOperatorTracking(r.Context(), params.SupportedTacs); err != nil {
			logger.APILog.Warn("Failed to update operator tracking information", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to update operator tracking information", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator tracking information updated successfully"}
		writeResponse(w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			UpdateOperatorTrackingAction,
			email,
			getClientIP(r),
			"User updated operator tracking information",
		)
	})
}

func UpdateOperatorID(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorIDParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}
		if params.Mcc == "" {
			writeError(w, http.StatusBadRequest, "mcc is missing", nil, logger.APILog)
			return
		}
		if params.Mnc == "" {
			writeError(w, http.StatusBadRequest, "mnc is missing", nil, logger.APILog)
			return
		}
		if !isValidMcc(params.Mcc) {
			writeError(w, http.StatusBadRequest, "Invalid mcc format. Must be a 3-decimal digit.", nil, logger.APILog)
			return
		}
		if !isValidMnc(params.Mnc) {
			writeError(w, http.StatusBadRequest, "Invalid mnc format. Must be a 2 or 3-decimal digit.", nil, logger.APILog)
			return
		}

		numSubs, err := dbInstance.NumSubscribers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to get number of subscribers", err, logger.APILog)
			return
		}
		if numSubs > 0 {
			writeError(w, http.StatusBadRequest, "Cannot update operator ID when there are subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorID(r.Context(), params.Mcc, params.Mnc); err != nil {
			logger.APILog.Warn("Failed to update operator ID", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to update operatorID", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator ID updated successfully"}
		writeResponse(w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			UpdateOperatorIDAction,
			email,
			getClientIP(r),
			"User updated operator with Id: "+params.Mcc+params.Mnc,
		)
	})
}

func UpdateOperatorCode(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorCodeParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.OperatorCode == "" {
			writeError(w, http.StatusBadRequest, "operator code is missing", nil, logger.APILog)
			return
		}
		if !isValidOperatorCode(params.OperatorCode) {
			writeError(w, http.StatusBadRequest, "Invalid operator code format. Must be a 32-character hexadecimal string.", nil, logger.APILog)
			return
		}

		numSubs, err := dbInstance.NumSubscribers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to get number of subscribers", err, logger.APILog)
			return
		}
		if numSubs > 0 {
			writeError(w, http.StatusBadRequest, "Cannot update operator code when there are subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorCode(r.Context(), params.OperatorCode); err != nil {
			logger.APILog.Warn("Failed to update operator code", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to update operatorID", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Operator Code updated successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			UpdateOperatorCodeAction,
			email,
			getClientIP(r),
			"User updated operator Code",
		)
	})
}

func UpdateOperatorHomeNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorHomeNetworkParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.PrivateKey == "" {
			writeError(w, http.StatusBadRequest, "privateKey is missing", nil, logger.APILog)
			return
		}

		if !isValidPrivateKey(params.PrivateKey) {
			writeError(w, http.StatusBadRequest, "Invalid private key format. Must be a 32-byte hexadecimal string.", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateHomeNetworkPrivateKey(r.Context(), params.PrivateKey); err != nil {
			logger.APILog.Warn("Failed to update home network private key", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to update home network private key", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Home Network private key updated successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			UpdateOperatorHomeNetworkAction,
			email,
			getClientIP(r),
			"User updated home network private key",
		)
	})
}
