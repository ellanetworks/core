package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
)

func setInventoryCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE")
}

func ListRadios(c *gin.Context) {
	setInventoryCorsHeader(c)
	dbGnbs, err := queries.ListInventoryRadios()
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	radios := make([]*models.Gnb, 0)
	for _, dbGnb := range dbGnbs {
		radio := models.Gnb{
			Name: dbGnb.Name,
			Tac:  dbGnb.Tac,
		}
		radios = append(radios, &radio)
	}
	c.JSON(http.StatusOK, radios)
}

func CreateRadio(c *gin.Context) {
	setInventoryCorsHeader(c)
	if err := handleCreateRadio(c); err == nil {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func DeleteRadio(c *gin.Context) {
	setInventoryCorsHeader(c)
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
	logger.NmsLog.Infof("Received gNB %v", radioName)
	var err error
	var newGnb models.Gnb

	allowHeader := strings.Split(c.GetHeader("Content-Type"), ";")
	switch allowHeader[0] {
	case "application/json":
		err = c.ShouldBindJSON(&newGnb)
	}
	if err != nil {
		logger.NmsLog.Errorf(err.Error())
		return fmt.Errorf("failed to create gNB %v: %v", radioName, err)
	}
	if newGnb.Tac == "" {
		errorMessage := "tac is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	req := httpwrapper.NewRequest(c.Request, newGnb)
	procReq := req.Body.(models.Gnb)
	procReq.Name = radioName
	dbRadio := &dbModels.Radio{
		Name: procReq.Name,
		Tac:  procReq.Tac,
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
