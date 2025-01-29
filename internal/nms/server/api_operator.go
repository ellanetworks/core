package server

import (
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

type UpdateOperatorSliceParams struct {
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
}

type UpdateOperatorTrackingParams struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type UpdateOperatorIdParams struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type UpdateOperatorCodeParams struct {
	OperatorCode string `json:"operatorCode,omitempty"`
}

type GetOperatorTrackingResponse struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type GetOperatorResponse struct {
	Id       GetOperatorIdResponse       `json:"id,omitempty"`
	Slice    GetOperatorSliceResponse    `json:"slice,omitempty"`
	Tracking GetOperatorTrackingResponse `json:"tracking,omitempty"`
}

type GetOperatorSliceResponse struct {
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
}

type GetOperatorIdResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

const (
	GetOperatorAction            = "get_operator"
	GetOperatorSliceAction       = "get_operator_slice"
	GetOperatorTrackingAction    = "get_operator_tracking"
	GetOperatorIdAction          = "get_operator_id"
	UpdateOperatorSliceAction    = "update_operator_slice"
	UpdateOperatorTrackingAction = "update_operator_tracking"
	UpdateOperatorIdAction       = "update_operator_id"
	UpdateOperatorCodeAction     = "update_operator_code"
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
		logger.NmsLog.Warnln("Invalid operator code length: ", len(operatorCode))
		return false
	}
	_, err := hex.DecodeString(operatorCode)
	if err != nil {
		logger.NmsLog.Warnf("Invalid operator code: %s. Error: %v", operatorCode, err)
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
		dbOperator, err := dbInstance.GetOperator()
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Operator not found")
			return
		}

		operatorSlice := &GetOperatorResponse{
			Id: GetOperatorIdResponse{
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
		}

		err = writeResponse(c.Writer, operatorSlice, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetOperatorAction,
			email,
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
		dbOperator, err := dbInstance.GetOperator()
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Operator not found")
			return
		}

		operatorSlice := &GetOperatorSliceResponse{
			Sst: int(dbOperator.Sst),
			Sd:  dbOperator.Sd,
		}

		err = writeResponse(c.Writer, operatorSlice, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetOperatorSliceAction,
			email,
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
		dbOperator, err := dbInstance.GetOperator()
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Operator not found")
			return
		}

		operatorTracking := &GetOperatorTrackingResponse{
			SupportedTacs: dbOperator.GetSupportedTacs(),
		}

		err = writeResponse(c.Writer, operatorTracking, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetOperatorTrackingAction,
			email,
			"User retrieved operator tracking information",
		)
	}
}

func GetOperatorId(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbOperator, err := dbInstance.GetOperator()
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Operator not found")
			return
		}

		operatorId := &GetOperatorIdResponse{
			Mcc: dbOperator.Mcc,
			Mnc: dbOperator.Mnc,
		}

		err = writeResponse(c.Writer, operatorId, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetOperatorIdAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}

		if updateOperatorSliceParams.Sst == 0 {
			writeError(c.Writer, http.StatusBadRequest, "sst is missing")
			return
		}
		if updateOperatorSliceParams.Sd == 0 {
			writeError(c.Writer, http.StatusBadRequest, "sd is missing")
			return
		}

		if !isValidSst(updateOperatorSliceParams.Sst) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer")
			return
		}
		if !isValidSd(updateOperatorSliceParams.Sd) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit integer")
			return
		}

		err = dbInstance.UpdateOperatorSlice(int32(updateOperatorSliceParams.Sst), updateOperatorSliceParams.Sd)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update operator slice information")
			return
		}
		message := SuccessResponse{Message: "Operator slice information updated successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateOperatorSliceAction,
			email,
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
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if len(updateOperatorTrackingParams.SupportedTacs) == 0 {
			writeError(c.Writer, http.StatusBadRequest, "supportedTacs is missing")
			return
		}

		for _, tac := range updateOperatorTrackingParams.SupportedTacs {
			if !isValidTac(tac) {
				writeError(c.Writer, http.StatusBadRequest, "Invalid TAC format. Must be a 3-digit number")
				return
			}
		}

		err = dbInstance.UpdateOperatorTracking(updateOperatorTrackingParams.SupportedTacs)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update operator tracking information")
			return
		}
		message := SuccessResponse{Message: "Operator tracking information updated successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateOperatorTrackingAction,
			email,
			"User updated operator tracking information",
		)
	}
}

func UpdateOperatorId(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorIdParams UpdateOperatorIdParams
		err := c.ShouldBindJSON(&updateOperatorIdParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateOperatorIdParams.Mcc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mcc is missing")
			return
		}
		if updateOperatorIdParams.Mnc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mnc is missing")
			return
		}
		if !isValidMcc(updateOperatorIdParams.Mcc) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mcc format. Must be a 3-decimal digit.")
			return
		}
		if !isValidMnc(updateOperatorIdParams.Mnc) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mnc format. Must be a 2 or 3-decimal digit.")
			return
		}
		numSubs, err := dbInstance.NumSubscribers()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to get number of subscribers")
			return
		}
		if numSubs > 0 {
			writeError(c.Writer, http.StatusBadRequest, "Cannot update operator ID when there are subscribers")
			return
		}

		err = dbInstance.UpdateOperatorId(updateOperatorIdParams.Mcc, updateOperatorIdParams.Mnc)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update operatorId")
			return
		}
		message := SuccessResponse{Message: "Operator ID updated successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateOperatorIdAction,
			email,
			"User updated operator with Id: "+updateOperatorIdParams.Mcc+""+updateOperatorIdParams.Mnc,
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
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateOperatorCodeParams.OperatorCode == "" {
			writeError(c.Writer, http.StatusBadRequest, "operator code is missing")
			return
		}

		if !isValidOperatorCode(updateOperatorCodeParams.OperatorCode) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid operator code format. Must be a 32-character hexadecimal string.")
			return
		}

		numSubs, err := dbInstance.NumSubscribers()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to get number of subscribers")
			return
		}
		if numSubs > 0 {
			writeError(c.Writer, http.StatusBadRequest, "Cannot update operator code when there are subscribers")
			return
		}

		err = dbInstance.UpdateOperatorCode(updateOperatorCodeParams.OperatorCode)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update operatorId")
			return
		}
		message := SuccessResponse{Message: "Operator Code updated successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateOperatorCodeAction,
			email,
			"User updated operator Code",
		)
	}
}
