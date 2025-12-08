package server

import (
	"crypto/ecdh"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	MaxSupportedTACs = 12
)

type UpdateOperatorSliceParams struct {
	Sst int    `json:"sst,omitempty"`
	Sd  string `json:"sd,omitempty"`
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
	Sst int    `json:"sst,omitempty"`
	Sd  string `json:"sd,omitempty"`
}

type GetOperatorIDResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetOperatorHomeNetworkResponse struct {
	PublicKey string `json:"publicKey,omitempty"`
}

const (
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
	privateKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		logger.EllaLog.Warn("Failed to decode private key from hex", zap.Error(err))
		return false
	}

	_, err = ecdh.X25519().NewPrivateKey(privateKeyBytes)
	if err != nil {
		logger.EllaLog.Warn("Failed to create X25519 private key", zap.Error(err))
		return false
	}

	return true
}

func isValidTac(s string) bool {
	if len(s) != 6 {
		return false
	}

	if _, err := hex.DecodeString(s); err != nil {
		return false
	}

	return true
}

// SST is an 8-bit integer
func isValidSst(sst int) bool {
	return sst >= 0 && sst <= 0xFF
}

func ParseSDString(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return nil, fmt.Errorf("SD must not start with 0x")
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	if len(b) > 3 {
		return nil, fmt.Errorf("SD must be at most 3 bytes (6 hex characters)")
	}

	arr := make([]byte, 3)
	copy(arr, b)

	return arr, nil
}

func SDToString(sd []byte) string {
	if sd == nil {
		return ""
	}

	s := fmt.Sprintf("%02x%02x%02x", sd[0], sd[1], sd[2])

	return s
}

func GetOperator(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				Sd:  SDToString(dbOperator.Sd),
			},
			Tracking: GetOperatorTrackingResponse{
				SupportedTacs: dbOperator.GetSupportedTacs(),
			},
			HomeNetwork: GetOperatorHomeNetworkResponse{
				PublicKey: hnPublicKey,
			},
		}

		writeResponse(w, operator, http.StatusOK, logger.APILog)
	})
}

func GetOperatorSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorSlice := &GetOperatorSliceResponse{
			Sst: int(dbOperator.Sst),
			Sd:  SDToString(dbOperator.Sd),
		}

		writeResponse(w, operatorSlice, http.StatusOK, logger.APILog)
	})
}

func GetOperatorTracking(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorTracking := &GetOperatorTrackingResponse{
			SupportedTacs: dbOperator.GetSupportedTacs(),
		}

		writeResponse(w, operatorTracking, http.StatusOK, logger.APILog)
	})
}

func GetOperatorID(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		if !isValidSst(params.Sst) {
			writeError(w, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer", nil, logger.APILog)
			return
		}

		sd, err := ParseSDString(params.Sd)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit hex string", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorSlice(r.Context(), int32(params.Sst), sd); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update operator slice information", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator slice information updated successfully"}

		writeResponse(w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
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

		if len(params.SupportedTacs) > MaxSupportedTACs {
			writeError(w, http.StatusBadRequest, "Too many supported TACs. Maximum is "+strconv.Itoa(MaxSupportedTACs), nil, logger.APILog)
			return
		}

		for _, tac := range params.SupportedTacs {
			if !isValidTac(tac) {
				writeError(w, http.StatusBadRequest, "Invalid TAC format. Must be a 3 bytes hex string", nil, logger.APILog)
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
			r.Context(),
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

		numSubs, err := dbInstance.CountSubscribers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count subscribers", err, logger.APILog)
			return
		}

		if numSubs > 0 {
			writeError(w, http.StatusBadRequest, "Cannot update operator ID when there are subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorID(r.Context(), params.Mcc, params.Mnc); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update operatorID", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator ID updated successfully"}
		writeResponse(w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
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

		numSubs, err := dbInstance.CountSubscribers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count subscribers", err, logger.APILog)
			return
		}

		if numSubs > 0 {
			writeError(w, http.StatusBadRequest, "Cannot update operator code when there are subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorCode(r.Context(), params.OperatorCode); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update operatorID", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Operator Code updated successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
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
			r.Context(),
			UpdateOperatorHomeNetworkAction,
			email,
			getClientIP(r),
			"User updated home network private key",
		)
	})
}
