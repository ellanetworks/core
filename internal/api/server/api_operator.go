package server

import (
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

type UpdateOperatorNASSecurityParams struct {
	Ciphering []string `json:"ciphering"`
	Integrity []string `json:"integrity"`
}

type UpdateOperatorSPNParams struct {
	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`
}

type GetOperatorTrackingResponse struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type GetOperatorSPNResponse struct {
	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`
}

type GetOperatorResponse struct {
	ID              GetOperatorIDResponse          `json:"id,omitempty"`
	Slice           GetOperatorSliceResponse       `json:"slice,omitempty"`
	Tracking        GetOperatorTrackingResponse    `json:"tracking,omitempty"`
	HomeNetworkKeys []HomeNetworkKeyResponse       `json:"homeNetworkKeys"`
	NASSecurity     GetOperatorNASSecurityResponse `json:"nasSecurity"`
	SPN             GetOperatorSPNResponse         `json:"spn"`
}

type GetOperatorSliceResponse struct {
	Sst int    `json:"sst,omitempty"`
	Sd  string `json:"sd,omitempty"`
}

type GetOperatorIDResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetOperatorNASSecurityResponse struct {
	Ciphering []string `json:"ciphering"`
	Integrity []string `json:"integrity"`
}

const (
	UpdateOperatorSliceAction       = "update_operator_slice"
	UpdateOperatorTrackingAction    = "update_operator_tracking"
	UpdateOperatorIDAction          = "update_operator_id"
	UpdateOperatorCodeAction        = "update_operator_code"
	UpdateOperatorNASSecurityAction = "update_operator_nas_security"
	UpdateOperatorSPNAction         = "update_operator_spn"
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
		logger.APILog.Warn("Invalid operator code: not valid hex", zap.Error(err))
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
			writeError(r.Context(), w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		hnKeys, err := dbInstance.ListHomeNetworkKeys(r.Context())
		if err != nil {
			logger.APILog.Warn("Failed to list home network keys", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list home network keys", err, logger.APILog)

			return
		}

		keyResponses := make([]HomeNetworkKeyResponse, 0, len(hnKeys))
		for _, k := range hnKeys {
			pubKey, err := k.GetPublicKey()
			if err != nil {
				logger.APILog.Warn("Failed to derive public key", zap.Int("id", k.ID), zap.Error(err))
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to derive public key", err, logger.APILog)

				return
			}

			keyResponses = append(keyResponses, HomeNetworkKeyResponse{
				ID:            k.ID,
				KeyIdentifier: k.KeyIdentifier,
				Scheme:        k.Scheme,
				PublicKey:     pubKey,
			})
		}

		supportedTACs, err := dbOperator.GetSupportedTacs()
		if err != nil {
			logger.APILog.Warn("Failed to get supported TACs", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get supported TACs", err, logger.APILog)

			return
		}

		cipheringOrder, err := dbOperator.GetCiphering()
		if err != nil {
			logger.APILog.Warn("Failed to get ciphering order", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get ciphering order", err, logger.APILog)

			return
		}

		integrityOrder, err := dbOperator.GetIntegrity()
		if err != nil {
			logger.APILog.Warn("Failed to get integrity order", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get integrity order", err, logger.APILog)

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
				SupportedTacs: supportedTACs,
			},
			HomeNetworkKeys: keyResponses,
			NASSecurity: GetOperatorNASSecurityResponse{
				Ciphering: cipheringOrder,
				Integrity: integrityOrder,
			},
			SPN: GetOperatorSPNResponse{
				FullName:  dbOperator.SpnFullName,
				ShortName: dbOperator.SpnShortName,
			},
		}

		writeResponse(r.Context(), w, operator, http.StatusOK, logger.APILog)
	})
}

// Deprecated: Use GET /api/v1/operator instead, which returns the full operator
// configuration including slice data. This endpoint will be removed in a future release.
func GetOperatorSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorSlice := &GetOperatorSliceResponse{
			Sst: int(dbOperator.Sst),
			Sd:  SDToString(dbOperator.Sd),
		}

		writeResponse(r.Context(), w, operatorSlice, http.StatusOK, logger.APILog)
	})
}

// Deprecated: Use GET /api/v1/operator instead, which returns the full operator
// configuration including tracking data. This endpoint will be removed in a future release.
func GetOperatorTracking(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		supportedTACs, err := dbOperator.GetSupportedTacs()
		if err != nil {
			logger.APILog.Warn("Failed to get supported TACs", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get supported TACs", err, logger.APILog)

			return
		}

		operatorTracking := &GetOperatorTrackingResponse{
			SupportedTacs: supportedTACs,
		}

		writeResponse(r.Context(), w, operatorTracking, http.StatusOK, logger.APILog)
	})
}

// Deprecated: Use GET /api/v1/operator instead, which returns the full operator
// configuration including the PLMN ID. This endpoint will be removed in a future release.
func GetOperatorID(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbOperator, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Operator not found", err, logger.APILog)
			return
		}

		operatorID := &GetOperatorIDResponse{
			Mcc: dbOperator.Mcc,
			Mnc: dbOperator.Mnc,
		}

		writeResponse(r.Context(), w, operatorID, http.StatusOK, logger.APILog)
	})
}

func UpdateOperatorSlice(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorSliceParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Sst == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "sst is missing", nil, logger.APILog)
			return
		}

		if !isValidSst(params.Sst) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer", nil, logger.APILog)
			return
		}

		sd, err := ParseSDString(params.Sd)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit hex string", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorSlice(r.Context(), int32(params.Sst), sd); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update operator slice information", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator slice information updated successfully"}

		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

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
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorTrackingParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if len(params.SupportedTacs) == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "supportedTacs is missing", nil, logger.APILog)
			return
		}

		if len(params.SupportedTacs) > MaxSupportedTACs {
			writeError(r.Context(), w, http.StatusBadRequest, "Too many supported TACs. Maximum is "+strconv.Itoa(MaxSupportedTACs), nil, logger.APILog)
			return
		}

		for _, tac := range params.SupportedTacs {
			if !isValidTac(tac) {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid TAC format. Must be a 3 bytes hex string", nil, logger.APILog)
				return
			}
		}

		if err := dbInstance.UpdateOperatorTracking(r.Context(), params.SupportedTacs); err != nil {
			logger.APILog.Warn("Failed to update operator tracking information", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update operator tracking information", err, logger.APILog)

			return
		}

		resp := SuccessResponse{Message: "Operator tracking information updated successfully"}
		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

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
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorIDParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Mcc == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "mcc is missing", nil, logger.APILog)
			return
		}

		if params.Mnc == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "mnc is missing", nil, logger.APILog)
			return
		}

		if !isValidMcc(params.Mcc) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid mcc format. Must be a 3-decimal digit.", nil, logger.APILog)
			return
		}

		if !isValidMnc(params.Mnc) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid mnc format. Must be a 2 or 3-decimal digit.", nil, logger.APILog)
			return
		}

		numSubs, err := dbInstance.CountSubscribers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count subscribers", err, logger.APILog)
			return
		}

		if numSubs > 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "Cannot update operator ID when there are subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorID(r.Context(), params.Mcc, params.Mnc); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update operatorID", err, logger.APILog)
			return
		}

		resp := SuccessResponse{Message: "Operator ID updated successfully"}
		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

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
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorCodeParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.OperatorCode == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "operator code is missing", nil, logger.APILog)
			return
		}

		if !isValidOperatorCode(params.OperatorCode) {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid operator code format. Must be a 32-character hexadecimal string.", nil, logger.APILog)
			return
		}

		numSubs, err := dbInstance.CountSubscribers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count subscribers", err, logger.APILog)
			return
		}

		if numSubs > 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "Cannot update operator code when there are subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorCode(r.Context(), params.OperatorCode); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update operatorID", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Operator Code updated successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateOperatorCodeAction,
			email,
			getClientIP(r),
			"User updated operator Code",
		)
	})
}

var validCipheringAlgorithms = map[string]bool{"NEA0": true, "NEA1": true, "NEA2": true}

var validIntegrityAlgorithms = map[string]bool{"NIA0": true, "NIA1": true, "NIA2": true}

func isValidAlgorithmOrder(order []string, valid map[string]bool) (string, bool) {
	if len(order) == 0 {
		return "", false
	}

	if len(order) > 3 {
		return "", false
	}

	seen := make(map[string]bool, len(order))

	for _, alg := range order {
		if !valid[alg] {
			return alg, false
		}

		if seen[alg] {
			return alg, false
		}

		seen[alg] = true
	}

	return "", true
}

func UpdateOperatorNASSecurity(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorNASSecurityParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if len(params.Ciphering) == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "ciphering is required and must not be empty", nil, logger.APILog)
			return
		}

		if len(params.Integrity) == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "integrity is required and must not be empty", nil, logger.APILog)
			return
		}

		if badAlg, valid := isValidAlgorithmOrder(params.Ciphering, validCipheringAlgorithms); !valid {
			if badAlg != "" {
				writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("Invalid or duplicate ciphering algorithm: %s. Allowed: NEA0, NEA1, NEA2", badAlg), nil, logger.APILog)
			} else {
				writeError(r.Context(), w, http.StatusBadRequest, "Maximum 3 ciphering algorithms allowed", nil, logger.APILog)
			}

			return
		}

		if badAlg, valid := isValidAlgorithmOrder(params.Integrity, validIntegrityAlgorithms); !valid {
			if badAlg != "" {
				writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("Invalid or duplicate integrity algorithm: %s. Allowed: NIA0, NIA1, NIA2", badAlg), nil, logger.APILog)
			} else {
				writeError(r.Context(), w, http.StatusBadRequest, "Maximum 3 integrity algorithms allowed", nil, logger.APILog)
			}

			return
		}

		if err := dbInstance.UpdateOperatorSecurityAlgorithms(r.Context(), params.Ciphering, params.Integrity); err != nil {
			logger.APILog.Warn("Failed to update operator security algorithms", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update operator security algorithms", err, logger.APILog)

			return
		}

		resp := SuccessResponse{Message: "Operator NAS security algorithms updated successfully"}
		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateOperatorNASSecurityAction,
			email,
			getClientIP(r),
			"User updated operator NAS security algorithms",
		)
	})
}

const (
	maxSPNLength = 50
)

func UpdateOperatorSPN(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateOperatorSPNParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.FullName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "fullName is required and must not be empty", nil, logger.APILog)
			return
		}

		if params.ShortName == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "shortName is required and must not be empty", nil, logger.APILog)
			return
		}

		if len(params.FullName) > maxSPNLength {
			writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("fullName must be at most %d characters", maxSPNLength), nil, logger.APILog)
			return
		}

		if len(params.ShortName) > maxSPNLength {
			writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("shortName must be at most %d characters", maxSPNLength), nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateOperatorSPN(r.Context(), params.FullName, params.ShortName); err != nil {
			logger.APILog.Warn("Failed to update operator SPN", zap.Error(err))
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update operator SPN", err, logger.APILog)

			return
		}

		resp := SuccessResponse{Message: "Operator SPN updated successfully"}
		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateOperatorSPNAction,
			email,
			getClientIP(r),
			"User updated operator SPN",
		)
	})
}
