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

func GetGnbs(c *gin.Context) {
	setInventoryCorsHeader(c)
	dbGnbs, err := queries.ListInventoryGnbs()
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	gnbs := make([]*models.Gnb, 0)
	for _, dbGnb := range dbGnbs {
		gnb := models.Gnb{
			Name: dbGnb.Name,
			Tac:  dbGnb.Tac,
		}
		gnbs = append(gnbs, &gnb)
	}
	c.JSON(http.StatusOK, gnbs)
}

func PostGnb(c *gin.Context) {
	setInventoryCorsHeader(c)
	if err := handlePostGnb(c); err == nil {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func DeleteGnb(c *gin.Context) {
	setInventoryCorsHeader(c)
	if err := handleDeleteGnb(c); err == nil {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func handlePostGnb(c *gin.Context) error {
	var gnbName string
	var exists bool
	if gnbName, exists = c.Params.Get("gnb-name"); !exists {
		errorMessage := "gnb-name is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	logger.NmsLog.Infof("Received gNB %v", gnbName)
	var err error
	var newGnb models.Gnb

	allowHeader := strings.Split(c.GetHeader("Content-Type"), ";")
	switch allowHeader[0] {
	case "application/json":
		err = c.ShouldBindJSON(&newGnb)
	}
	if err != nil {
		logger.NmsLog.Errorf(err.Error())
		return fmt.Errorf("failed to create gNB %v: %v", gnbName, err)
	}
	if newGnb.Tac == "" {
		errorMessage := "tac is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	req := httpwrapper.NewRequest(c.Request, newGnb)
	procReq := req.Body.(models.Gnb)
	procReq.Name = gnbName
	dbGnb := &dbModels.Gnb{
		Name: procReq.Name,
		Tac:  procReq.Tac,
	}
	queries.CreateGnb(dbGnb)
	logger.NmsLog.Infof("created gnb %v", gnbName)
	return nil
}

func handleDeleteGnb(c *gin.Context) error {
	var gnbName string
	var exists bool
	if gnbName, exists = c.Params.Get("gnb-name"); !exists {
		errorMessage := "gnb-name is missing"
		logger.NmsLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	err := queries.DeleteGnb(gnbName)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	logger.NmsLog.Infof("Deleted gnb %v", gnbName)
	return nil
}
