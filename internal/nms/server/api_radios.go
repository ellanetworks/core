package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

type CreateRadioParams struct {
	Name string `json:"name"`
}

type PlmnId struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Tai struct {
	PlmnId PlmnId `json:"plmnId"`
	Tac    string `json:"tac"`
}

type Snssai struct {
	Sst int32  `json:"sst"`
	Sd  string `json:"sd"`
}

type SupportedTAI struct {
	Tai     Tai      `json:"tai"`
	SNssais []Snssai `json:"snssais"`
}

type GetRadioParams struct {
	Name          string         `json:"name"`
	Id            string         `json:"id"`
	IPAddress     string         `json:"ip_address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

const (
	ListRadiosAction  = "list_radios"
	GetRadioAction    = "get_radio"
	CreateRadioAction = "create_radio"
	UpdateRadioAction = "update_radio"
	DeleteRadioAction = "delete_radio"
)

func isValidRadioName(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func convertRadioTaiToReturnTai(tais []context.SupportedTAI) []SupportedTAI {
	returnedTais := make([]SupportedTAI, 0)
	for _, tai := range tais {
		snssais := make([]Snssai, 0)
		for _, snssai := range tai.SNssaiList {
			newSnssai := Snssai{
				Sst: snssai.Sst,
				Sd:  snssai.Sd,
			}
			snssais = append(snssais, newSnssai)
		}
		newTai := SupportedTAI{
			Tai: Tai{
				PlmnId: PlmnId{
					Mcc: tai.Tai.PlmnId.Mcc,
					Mnc: tai.Tai.PlmnId.Mnc,
				},
				Tac: tai.Tai.Tac,
			},
			SNssais: snssais,
		}
		returnedTais = append(returnedTais, newTai)
	}
	return returnedTais
}

func ListRadios(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}

		ranList := context.ListAmfRan()
		radios := make([]GetRadioParams, 0)
		for _, radio := range ranList {
			supportedTais := convertRadioTaiToReturnTai(radio.SupportedTAList)
			newRadio := GetRadioParams{
				Name:          radio.Name,
				Id:            radio.GnbId,
				IPAddress:     radio.GnbIp,
				SupportedTAIs: supportedTais,
			}
			radios = append(radios, newRadio)
		}

		err := writeResponse(c.Writer, radios, http.StatusOK)
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

// func GetRadio(dbInstance *db.Database) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		emailAny, _ := c.Get("email")
// 		email, ok := emailAny.(string)
// 		if !ok {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
// 			return
// 		}
// 		radioName := c.Param("name")
// 		if radioName == "" {
// 			writeError(c.Writer, http.StatusBadRequest, "Missing name parameter")
// 			return
// 		}
// 		// dbRadio, err := dbInstance.GetRadio(radioName)
// 		// if err != nil {
// 		// 	writeError(c.Writer, http.StatusNotFound, "Radio not found")
// 		// 	return
// 		// }

// 		// radio := GetRadioParams{
// 		// 	Name: dbRadio.Name,
// 		// }
// 		ranList := context.ListAmfRan()
// 		for _, radio := range ranList {

// 		}
// 		err = writeResponse(c.Writer, radio, http.StatusOK)
// 		if err != nil {
// 			writeError(c.Writer, http.StatusInternalServerError, "internal error")
// 			return
// 		}
// 		logger.LogAuditEvent(
// 			GetRadioAction,
// 			email,
// 			"User retrieved radio: "+radioName,
// 		)
// 	}
// }

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
		if !isValidRadioName(newRadio.Name) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid name format. Must be less than 256 characters")
			return
		}
		_, err = dbInstance.GetRadio(newRadio.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "radio already exists")
			return
		}

		dbRadio := &db.Radio{
			Name: newRadio.Name,
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
