package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"go.mongodb.org/mongo-driver/bson"
)

func setInventoryCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE")
}

func GetGnbs(c *gin.Context) {
	setInventoryCorsHeader(c)
	logger.NMSLog.Infoln("Get all gNBs")

	var gnbs []*models.Gnb
	gnbs = make([]*models.Gnb, 0)
	rawGnbs, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.GnbDataColl, bson.M{})
	if errGetMany != nil {
		logger.DbLog.Errorln(errGetMany)
		c.JSON(http.StatusInternalServerError, gnbs)
	}

	for _, rawGnb := range rawGnbs {
		var gnbData models.Gnb
		err := json.Unmarshal(mapToByte(rawGnb), &gnbData)
		if err != nil {
			logger.DbLog.Errorf("Could not unmarshall gNB %v", rawGnb)
		}
		gnbs = append(gnbs, &gnbData)
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
		logger.ConfigLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	logger.ConfigLog.Infof("Received gNB %v", gnbName)
	var err error
	var newGnb models.Gnb

	allowHeader := strings.Split(c.GetHeader("Content-Type"), ";")
	switch allowHeader[0] {
	case "application/json":
		err = c.ShouldBindJSON(&newGnb)
	}
	if err != nil {
		logger.ConfigLog.Errorf(err.Error())
		return fmt.Errorf("failed to create gNB %v: %v", gnbName, err)
	}
	if newGnb.Tac == "" {
		errorMessage := "tac is missing"
		logger.ConfigLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	req := httpwrapper.NewRequest(c.Request, newGnb)
	procReq := req.Body.(models.Gnb)
	procReq.Name = gnbName
	filter := bson.M{"name": gnbName}
	gnbDataBson := toBsonM(&procReq)
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.GnbDataColl, filter, gnbDataBson)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	logger.ConfigLog.Infof("created gnb %v", gnbName)
	return nil
}

func handleDeleteGnb(c *gin.Context) error {
	var gnbName string
	var exists bool
	if gnbName, exists = c.Params.Get("gnb-name"); !exists {
		errorMessage := "gnb-name is missing"
		logger.ConfigLog.Errorf(errorMessage)
		return errors.New(errorMessage)
	}
	logger.ConfigLog.Infof("Received delete gNB %v request", gnbName)
	filter := bson.M{"name": gnbName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.GnbDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	logger.ConfigLog.Infof("Deleted gnb %v", gnbName)
	return nil
}
