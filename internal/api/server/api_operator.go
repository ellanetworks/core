package server

import (
	"encoding/hex"
	"log"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
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

func GetOperator(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbOperator, err := dbInstance.GetOperator(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Operator not found")
			return
		}

		hnPublicKey, err := dbOperator.GetHomeNetworkPublicKey()
		if err != nil {
			logger.APILog.Warn("Failed to get home network public key", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to get home network public key")
			return
		}

		operatorSlice := &GetOperatorResponse{
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

		writeResponse(c, operatorSlice, http.StatusOK)
		logger.LogAuditEvent(
			GetOperatorAction,
			email,
			c.ClientIP(),
			"User retrieved operator information",
		)
	}
}

func GetOperatorSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbOperator, err := dbInstance.GetOperator(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Operator not found")
			return
		}

		operatorSlice := &GetOperatorSliceResponse{
			Sst: int(dbOperator.Sst),
			Sd:  dbOperator.Sd,
		}

		writeResponse(c, operatorSlice, http.StatusOK)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetOperatorSliceAction,
			email,
			c.ClientIP(),
			"User retrieved operator slice",
		)
	}
}

func GetOperatorTracking(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbOperator, err := dbInstance.GetOperator(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Operator not found")
			return
		}

		operatorTracking := &GetOperatorTrackingResponse{
			SupportedTacs: dbOperator.GetSupportedTacs(),
		}

		writeResponse(c, operatorTracking, http.StatusOK)
		logger.LogAuditEvent(
			GetOperatorTrackingAction,
			email,
			c.ClientIP(),
			"User retrieved operator tracking information",
		)
	}
}

func GetOperatorID(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbOperator, err := dbInstance.GetOperator(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Operator not found")
			return
		}

		operatorID := &GetOperatorIDResponse{
			Mcc: dbOperator.Mcc,
			Mnc: dbOperator.Mnc,
		}

		writeResponse(c, operatorID, http.StatusOK)
		logger.LogAuditEvent(
			GetOperatorIDAction,
			email,
			c.ClientIP(),
			"User retrieved operator Id",
		)
	}
}

func UpdateOperatorSlice(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorSliceParams UpdateOperatorSliceParams
		err := c.ShouldBindJSON(&updateOperatorSliceParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}

		if updateOperatorSliceParams.Sst == 0 {
			writeError(c, http.StatusBadRequest, "sst is missing")
			return
		}
		if updateOperatorSliceParams.Sd == 0 {
			writeError(c, http.StatusBadRequest, "sd is missing")
			return
		}

		if !isValidSst(updateOperatorSliceParams.Sst) {
			writeError(c, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer")
			return
		}
		if !isValidSd(updateOperatorSliceParams.Sd) {
			writeError(c, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit integer")
			return
		}

		err = dbInstance.UpdateOperatorSlice(int32(updateOperatorSliceParams.Sst), updateOperatorSliceParams.Sd, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update operator slice information", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update operator slice information")
			return
		}
		message := SuccessResponse{Message: "Operator slice information updated successfully"}
		writeResponse(c, message, http.StatusCreated)
		logger.LogAuditEvent(
			UpdateOperatorSliceAction,
			email,
			c.ClientIP(),
			"User updated operator slice information",
		)
	}
}

func UpdateOperatorTracking(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorTrackingParams UpdateOperatorTrackingParams
		err := c.ShouldBindJSON(&updateOperatorTrackingParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if len(updateOperatorTrackingParams.SupportedTacs) == 0 {
			writeError(c, http.StatusBadRequest, "supportedTacs is missing")
			return
		}

		for _, tac := range updateOperatorTrackingParams.SupportedTacs {
			if !isValidTac(tac) {
				writeError(c, http.StatusBadRequest, "Invalid TAC format. Must be a 3-digit number")
				return
			}
		}

		err = dbInstance.UpdateOperatorTracking(updateOperatorTrackingParams.SupportedTacs, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update operator tracking information", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update operator tracking information")
			return
		}
		message := SuccessResponse{Message: "Operator tracking information updated successfully"}
		writeResponse(c, message, http.StatusCreated)
		logger.LogAuditEvent(
			UpdateOperatorTrackingAction,
			email,
			c.ClientIP(),
			"User updated operator tracking information",
		)
	}
}

func UpdateOperatorID(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorIDParams UpdateOperatorIDParams
		err := c.ShouldBindJSON(&updateOperatorIDParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateOperatorIDParams.Mcc == "" {
			writeError(c, http.StatusBadRequest, "mcc is missing")
			return
		}
		if updateOperatorIDParams.Mnc == "" {
			writeError(c, http.StatusBadRequest, "mnc is missing")
			return
		}
		if !isValidMcc(updateOperatorIDParams.Mcc) {
			writeError(c, http.StatusBadRequest, "Invalid mcc format. Must be a 3-decimal digit.")
			return
		}
		if !isValidMnc(updateOperatorIDParams.Mnc) {
			writeError(c, http.StatusBadRequest, "Invalid mnc format. Must be a 2 or 3-decimal digit.")
			return
		}
		numSubs, err := dbInstance.NumSubscribers(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to get number of subscribers")
			return
		}
		if numSubs > 0 {
			writeError(c, http.StatusBadRequest, "Cannot update operator ID when there are subscribers")
			return
		}

		err = dbInstance.UpdateOperatorID(updateOperatorIDParams.Mcc, updateOperatorIDParams.Mnc, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update operator ID", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update operatorID")
			return
		}
		message := SuccessResponse{Message: "Operator ID updated successfully"}
		writeResponse(c, message, http.StatusCreated)
		logger.LogAuditEvent(
			UpdateOperatorIDAction,
			email,
			c.ClientIP(),
			"User updated operator with Id: "+updateOperatorIDParams.Mcc+""+updateOperatorIDParams.Mnc,
		)
	}
}

func UpdateOperatorCode(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorCodeParams UpdateOperatorCodeParams
		err := c.ShouldBindJSON(&updateOperatorCodeParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateOperatorCodeParams.OperatorCode == "" {
			writeError(c, http.StatusBadRequest, "operator code is missing")
			return
		}

		if !isValidOperatorCode(updateOperatorCodeParams.OperatorCode) {
			writeError(c, http.StatusBadRequest, "Invalid operator code format. Must be a 32-character hexadecimal string.")
			return
		}

		numSubs, err := dbInstance.NumSubscribers(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to get number of subscribers")
			return
		}
		if numSubs > 0 {
			writeError(c, http.StatusBadRequest, "Cannot update operator code when there are subscribers")
			return
		}

		err = dbInstance.UpdateOperatorCode(updateOperatorCodeParams.OperatorCode, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update operator code", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update operatorID")
			return
		}
		message := SuccessResponse{Message: "Operator Code updated successfully"}
		writeResponse(c, message, http.StatusCreated)
		logger.LogAuditEvent(
			UpdateOperatorCodeAction,
			email,
			c.ClientIP(),
			"User updated operator Code",
		)
	}
}

func UpdateOperatorHomeNetwork(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorHomeNetworkParams UpdateOperatorHomeNetworkParams
		err := c.ShouldBindJSON(&updateOperatorHomeNetworkParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateOperatorHomeNetworkParams.PrivateKey == "" {
			writeError(c, http.StatusBadRequest, "privateKey is missing")
			return
		}

		if !isValidPrivateKey(updateOperatorHomeNetworkParams.PrivateKey) {
			writeError(c, http.StatusBadRequest, "Invalid private key format. Must be a 32-byte hexadecimal string.")
			return
		}

		err = dbInstance.UpdateHomeNetworkPrivateKey(updateOperatorHomeNetworkParams.PrivateKey, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to update home network private key", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update home network private key")
			return
		}
		message := SuccessResponse{Message: "Home Network private key updated successfully"}
		writeResponse(c, message, http.StatusCreated)
		logger.LogAuditEvent(
			UpdateOperatorHomeNetworkAction,
			email,
			c.ClientIP(),
			"User updated home network private key",
		)
	}
}
