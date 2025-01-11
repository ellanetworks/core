package server

import (
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/gin-gonic/gin"
)

type UpdateOperatorParams struct {
	Mcc           string   `json:"mcc,omitempty"`
	Mnc           string   `json:"mnc,omitempty"`
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type UpdateOperatorCodeParams struct {
	OperatorCode string `json:"operatorCode,omitempty"`
}

type GetOperatorResponse struct {
	Mcc           string   `json:"mcc,omitempty"`
	Mnc           string   `json:"mnc,omitempty"`
	SupportedTacs []string `json:"supportedTacs,omitempty"`
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

		dbOperator := &db.Operator{
			Mcc: updateOperatorParams.Mcc,
			Mnc: updateOperatorParams.Mnc,
		}
		dbOperator.SetSupportedTacs(updateOperatorParams.SupportedTacs)

		err = dbInstance.UpdateOperator(dbOperator)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update operatorId")
			return
		}
		updateSMF(dbInstance)
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

func updateSMF(dbInstance *db.Database) {
	dbOperator, err := dbInstance.GetOperator()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	operator := &models.Network{
		Mcc: dbOperator.Mcc,
		Mnc: dbOperator.Mnc,
	}

	profiles := make([]models.Profile, 0)
	dbProfiles, err := dbInstance.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	for _, dbProfile := range dbProfiles {
		profile := models.Profile{
			Name:            dbProfile.Name,
			UeIpPool:        dbProfile.UeIpPool,
			Dns:             dbProfile.Dns,
			BitrateDownlink: dbProfile.BitrateDownlink,
			BitrateUplink:   dbProfile.BitrateUplink,
			Var5qi:          dbProfile.Var5qi,
			PriorityLevel:   dbProfile.PriorityLevel,
		}
		profiles = append(profiles, profile)
	}
	dbRadios, err := dbInstance.ListRadios()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	radios := make([]models.Radio, 0)
	for _, dbRadio := range dbRadios {
		radio := models.Radio{
			Name: dbRadio.Name,
		}
		radios = append(radios, radio)
	}
	context.UpdateSMFContext(operator, profiles, radios)
}
