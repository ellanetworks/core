package server

import (
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

type UpdateOperatorParams struct {
	Mcc           string   `json:"mcc,omitempty"`
	Mnc           string   `json:"mnc,omitempty"`
	SupportedTacs []string `json:"supportedTacs,omitempty"`
	Sst           int      `json:"sst,omitempty"`
	Sd            int      `json:"sd,omitempty"`
}

type UpdateOperatorCodeParams struct {
	OperatorCode string `json:"operatorCode,omitempty"`
}

type GetOperatorResponse struct {
	Mcc           string   `json:"mcc,omitempty"`
	Mnc           string   `json:"mnc,omitempty"`
	SupportedTacs []string `json:"supportedTacs,omitempty"`
	Sst           int      `json:"sst,omitempty"`
	Sd            int      `json:"sd,omitempty"`
}

const (
	GetOperatorAction        = "get_operator"
	UpdateOperatorAction     = "update_operator"
	UpdateOperatorCodeAction = "update_operator_code"
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
	}
	return err == nil
}

// TAC is a 24-bit identifier
func isValidTac(tac string) bool {
	if len(tac) != 3 {
		return false
	}
	_, err := strconv.Atoi(tac)
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

		operatorId := &GetOperatorResponse{
			Mcc:           dbOperator.Mcc,
			Mnc:           dbOperator.Mnc,
			SupportedTacs: dbOperator.GetSupportedTacs(),
			Sst:           int(dbOperator.Sst),
			Sd:            dbOperator.Sd,
		}

		err = writeResponse(c.Writer, operatorId, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetOperatorAction,
			email,
			"User retrieved operator",
		)
	}
}

func UpdateOperator(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var updateOperatorParams UpdateOperatorParams
		err := c.ShouldBindJSON(&updateOperatorParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateOperatorParams.Mcc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mcc is missing")
			return
		}
		if updateOperatorParams.Mnc == "" {
			writeError(c.Writer, http.StatusBadRequest, "mnc is missing")
			return
		}
		if len(updateOperatorParams.SupportedTacs) == 0 {
			writeError(c.Writer, http.StatusBadRequest, "supportedTacs is missing")
			return
		}
		if updateOperatorParams.Sst == 0 {
			writeError(c.Writer, http.StatusBadRequest, "sst is missing")
			return
		}
		if updateOperatorParams.Sd == 0 {
			writeError(c.Writer, http.StatusBadRequest, "sd is missing")
			return
		}
		if !isValidMcc(updateOperatorParams.Mcc) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mcc format. Must be a 3-decimal digit.")
			return
		}
		if !isValidMnc(updateOperatorParams.Mnc) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid mnc format. Must be a 2 or 3-decimal digit.")
			return
		}
		for _, tac := range updateOperatorParams.SupportedTacs {
			if !isValidTac(tac) {
				writeError(c.Writer, http.StatusBadRequest, "Invalid TAC format. Must be a 3-digit number")
				return
			}
		}
		if !isValidSst(updateOperatorParams.Sst) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid SST format. Must be an 8-bit integer")
			return
		}
		if !isValidSd(updateOperatorParams.Sd) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid SD format. Must be a 24-bit integer")
			return
		}

		dbOperator := &db.Operator{
			Mcc: updateOperatorParams.Mcc,
			Mnc: updateOperatorParams.Mnc,
			Sst: int32(updateOperatorParams.Sst),
			Sd:  updateOperatorParams.Sd,
		}
		dbOperator.SetSupportedTacs(updateOperatorParams.SupportedTacs)

		err = dbInstance.UpdateOperator(dbOperator)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update operatorId")
			return
		}
		message := SuccessResponse{Message: "Operator updated successfully"}
		err = writeResponse(c.Writer, message, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateOperatorAction,
			email,
			"User updated operator with Id: "+updateOperatorParams.Mcc+""+updateOperatorParams.Mnc,
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
