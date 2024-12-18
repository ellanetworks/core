package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
)

type CreateRadioParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type GetRadioParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

func ListRadios(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbRadios, err := dbInstance.ListRadios()
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve radios")
			return
		}

		var radios []GetRadioParams
		for _, radio := range dbRadios {
			radios = append(radios, GetRadioParams{
				Name: radio.Name,
				Tac:  strconv.Itoa(radio.Tac),
			})
		}
		err = writeResponse(c.Writer, radios, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
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
			Tac:  strconv.Itoa(dbRadio.Tac),
		}
		err = writeResponse(c.Writer, radio, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
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
		_, err = dbInstance.GetRadio(newRadio.Name)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "radio already exists")
			return
		}
		logger.NmsLog.Infof("Received radio %v", newRadio.Name)

		tacInt, err := strconv.Atoi(newRadio.Tac)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		dbRadio := &db.Radio{
			Name: newRadio.Name,
			Tac:  tacInt,
		}
		err = dbInstance.CreateRadio(dbRadio)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create radio")
			return
		}
		logger.NmsLog.Infof("created radio %v", newRadio.Name)
		successResponse := SuccessResponse{Message: "Radio created successfully"}
		err = writeResponse(c.Writer, successResponse, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
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
	}
}
