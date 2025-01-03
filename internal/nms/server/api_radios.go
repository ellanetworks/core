package server

import (
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

type CreateRadioParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type GetRadioParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

const (
	ListRadiosAction  = "list_radios"
	GetRadioAction    = "get_radio"
	CreateRadioAction = "create_radio"
	UpdateRadioAction = "update_radio"
	DeleteRadioAction = "delete_radio"
)

// TAC is a 24-bit identifier
func isValidTac(tac string) bool {
	if len(tac) != 3 {
		return false
	}
	_, err := strconv.Atoi(tac)
	return err == nil
}

func isValidRadioName(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func ListRadios(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbRadios, err := dbInstance.ListRadios()
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve radios")
			return
		}

		radios := make([]GetRadioParams, 0)
		for _, radio := range dbRadios {
			radios = append(radios, GetRadioParams{
				Name: radio.Name,
				Tac:  radio.Tac,
			})
		}
		err = writeResponse(c.Writer, radios, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			ListRadiosAction,
			email,
			"User listed radios",
		)
	}
}

func GetRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		radioName := c.Param("name")
		if radioName == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		logger.NmsLog.Infof("Received GET radio %v", radioName)
		dbRadio, err := dbInstance.GetRadio(radioName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Radio not found")
			return
		}

		radio := GetRadioParams{
			Name: dbRadio.Name,
			Tac:  dbRadio.Tac,
		}
		err = writeResponse(c.Writer, radio, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetRadioAction,
			email,
			"User retrieved radio: "+radioName,
		)
	}
}

func CreateRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var newRadio CreateRadioParams
		err := c.ShouldBindJSON(&newRadio)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if newRadio.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "name is missing")
			return
		}
		if newRadio.Tac == "" {
			writeError(c.Writer, http.StatusBadRequest, "tac is missing")
			return
		}
		if !isValidRadioName(newRadio.Name) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		if !isValidTac(newRadio.Tac) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid TAC format. Must be a 3-digit number")
			return
		}
		_, err = dbInstance.GetRadio(newRadio.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "radio already exists")
			return
		}

		dbRadio := &db.Radio{
			Name: newRadio.Name,
			Tac:  newRadio.Tac,
		}
		err = dbInstance.CreateRadio(dbRadio)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create radio")
			return
		}
		successResponse := SuccessResponse{Message: "Radio created successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			CreateRadioAction,
			email,
			"User created radio: "+newRadio.Name,
		)
	}
}

func UpdateRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		radioName := c.Param("name")
		if radioName == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		var updateRadioParams CreateRadioParams
		err := c.ShouldBindJSON(&updateRadioParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateRadioParams.Name == "" {
			writeError(c.Writer, http.StatusBadRequest, "name is missing")
			return
		}
		if updateRadioParams.Tac == "" {
			writeError(c.Writer, http.StatusBadRequest, "tac is missing")
			return
		}
		if !isValidRadioName(updateRadioParams.Name) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		if !isValidTac(updateRadioParams.Tac) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid TAC format. Must be a 3-digit number")
			return
		}
		_, err = dbInstance.GetRadio(radioName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Radio not found")
			return
		}

		dbRadio := &db.Radio{
			Name: updateRadioParams.Name,
			Tac:  updateRadioParams.Tac,
		}
		err = dbInstance.UpdateRadio(dbRadio)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update radio")
			return
		}
		successResponse := SuccessResponse{Message: "Radio updated successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateRadioAction,
			email,
			"User updated radio: "+radioName,
		)
	}
}

func DeleteRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		radioName := c.Param("name")
		if radioName == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
			return
		}
		_, err := dbInstance.GetRadio(radioName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Radio not found")
			return
		}
		err = dbInstance.DeleteRadio(radioName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to delete radio")
			return
		}

		successResponse := SuccessResponse{Message: "Radio deleted successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			DeleteRadioAction,
			email,
			"User deleted radio: "+radioName,
		)
	}
}
