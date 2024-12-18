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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve radios"})
			return
		}

		var radios []GetRadioParams
		for _, radio := range dbRadios {
			radios = append(radios, GetRadioParams{
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
		radioName := c.Param("name")
		if radioName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing name parameter"})
			return
		}
		logger.NmsLog.Infof("Received GET radio %v", radioName)
		radio, err := dbInstance.GetRadio(radioName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Radio not found"})
			return
		}

		c.JSON(http.StatusOK, GetRadioParams{
			Name: radio.Name,
			Tac:  strconv.Itoa(radio.Tac),
		})
	}
}

func CreateRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var newRadio CreateRadioParams
		err := c.ShouldBindJSON(&newRadio)
		if err != nil {
			logger.NmsLog.Errorf(err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}
		if newRadio.Name == "" {
			errorMessage := "name is missing"
			logger.NmsLog.Errorf(errorMessage)
			c.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
			return
		}
		if newRadio.Tac == "" {
			errorMessage := "tac is missing"
			logger.NmsLog.Errorf(errorMessage)
			c.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
			return
		}
		_, err = dbInstance.GetRadio(newRadio.Name)
		if err == nil {
			errorMessage := "radio already exists"
			logger.NmsLog.Errorf(errorMessage)
			c.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
			return
		}
		logger.NmsLog.Infof("Received radio %v", newRadio.Name)

		tacInt, err := strconv.Atoi(newRadio.Tac)
		if err != nil {
			logger.NmsLog.Errorf(err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}
		dbRadio := &db.Radio{
			Name: newRadio.Name,
			Tac:  tacInt,
		}
		err = dbInstance.CreateRadio(dbRadio)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create radio"})
			return
		}
		logger.NmsLog.Infof("created radio %v", newRadio.Name)
		c.JSON(http.StatusCreated, gin.H{"message": "Radio created successfully"})
	}
}

func DeleteRadio(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		radioName := c.Param("name")
		if radioName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing name parameter"})
			return
		}
		_, err := dbInstance.GetRadio(radioName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Radio not found"})
			return
		}
		err = dbInstance.DeleteRadio(radioName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete radio"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Radio deleted successfully"})
	}
}
