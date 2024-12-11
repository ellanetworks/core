package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
)

func setRadiosCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE")
}

func ListRadios(c *gin.Context) {
	setRadiosCorsHeader(c)
	dbRadios, err := queries.ListRadios()
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	radios := make([]*models.Radio, 0)
	for _, dbRadio := range dbRadios {
		radio := models.Radio{
			Name: dbRadio.Name,
			Tac:  string(dbRadio.Tac),
		}
		radios = append(radios, &radio)
	}
	c.JSON(http.StatusOK, radios)
}

func CreateRadio(c *gin.Context) {
	setRadiosCorsHeader(c)
	if err := handleCreateRadio(c); err == nil {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func DeleteRadio(c *gin.Context) {
	setRadiosCorsHeader(c)
	if err := handleDeleteRadio(c); err == nil {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func handleCreateRadio(c *gin.Context) error {
	var radioName string
	var exists bool
	if radioName, exists = c.Params.Get("radio-name"); !exists {
		errorMessage := "radio-name is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	logger.NmsLog.Infof("Received radio %v", radioName)
	var err error
	var newRadio models.Radio

	allowHeader := strings.Split(c.GetHeader("Content-Type"), ";")
	switch allowHeader[0] {
	case "application/json":
		err = c.ShouldBindJSON(&newRadio)
	}
	if err != nil {
		logger.NmsLog.Errorf(err.Error())
		return fmt.Errorf("failed to create radio %v: %v", radioName, err)
	}
	if newRadio.Tac == "" {
		errorMessage := "tac is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	req := httpwrapper.NewRequest(c.Request, newRadio)
	procReq := req.Body.(models.Radio)
	procReq.Name = radioName
	intTac, err := strconv.Atoi(procReq.Tac)
	if err != nil {
		return fmt.Errorf("failed to convert tac %v to int: %v", procReq.Tac, err)
	}
	dbRadio := &dbModels.Radio{
		Name: procReq.Name,
		Tac:  int32(intTac),
	}
	queries.CreateRadio(dbRadio)
	logger.NmsLog.Infof("created radio %v", radioName)
	return nil
}

func handleDeleteRadio(c *gin.Context) error {
	var radioName string
	var exists bool
	if radioName, exists = c.Params.Get("radio-name"); !exists {
		errorMessage := "radio-name is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	err := queries.DeleteRadio(radioName)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	logger.NmsLog.Infof("Deleted radio %v", radioName)
	return nil
}
