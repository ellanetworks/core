package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
)

func GetRadios(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbRadios, err := dbInstance.ListRadios()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve radios"})
			return
		}

		var radios []models.Radio
		for _, radio := range dbRadios {
			radios = append(radios, models.Radio{
				Name: radio.Name,
				Tac:  strconv.Itoa(radio.Tac),
			})
		}
		c.JSON(http.StatusOK, radios)
	}
}

func GetRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		radioName := c.Param("radio-name")
		if radioName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing radio-name parameter"})
			return
		}

		radio, err := dbInstance.GetRadioByName(radioName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Radio not found"})
			return
		}

		c.JSON(http.StatusOK, models.Radio{
			Name: radio.Name,
			Tac:  strconv.Itoa(radio.Tac),
		})
	}
}

func PostRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var radioName string
		var exists bool
		if radioName, exists = c.Params.Get("radio-name"); !exists {
			errorMessage := "radio-name is missing"
			logger.NmsLog.Errorf(errorMessage)
			c.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
			return
		}
		_, err := dbInstance.GetRadioByName(radioName)
		if err == nil {
			errorMessage := "radio already exists"
			logger.NmsLog.Errorf(errorMessage)
			c.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
			return
		}
		logger.NmsLog.Infof("Received radio %v", radioName)
		var newRadio models.Radio
		err = c.ShouldBindJSON(&newRadio)
		if err != nil {
			logger.NmsLog.Errorf(err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}
		if newRadio.Tac == "" {
			errorMessage := "tac is missing"
			logger.NmsLog.Errorf(errorMessage)
			c.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
			return
		}
		tacInt, err := strconv.Atoi(newRadio.Tac)
		if err != nil {
			logger.NmsLog.Errorf(err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}
		dbRadio := &db.Radio{
			Name: radioName,
			Tac:  tacInt,
		}
		err = dbInstance.CreateRadio(dbRadio)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create radio"})
			return
		}
		logger.NmsLog.Infof("created radio %v", radioName)
		c.JSON(http.StatusCreated, gin.H{"message": "Radio created successfully"})
	}
}

func DeleteRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		radioName := c.Param("radio-name")
		if radioName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing radio-name parameter"})
			return
		}

		radio, err := dbInstance.GetRadioByName(radioName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Radio not found"})
			return
		}

		err = dbInstance.DeleteRadio(radio.ID)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete radio"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Radio deleted successfully"})
	}
}
