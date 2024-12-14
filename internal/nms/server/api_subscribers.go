package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	nmsModels "github.com/yeastengine/ella/internal/nms/models"
)

func setCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
}

func GetSubscribers(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		subscribers, err := dbInstance.ListSubscribers()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve subscribers"})
			return
		}

		var subsList []nmsModels.SubsListIE
		for _, subscriber := range subscribers {
			subsList = append(subsList, nmsModels.SubsListIE{
				PlmnID: subscriber.PlmnID,
				UeId:   subscriber.UeId,
			})
		}
		c.JSON(http.StatusOK, subsList)
	}
}

func GetSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		ueId := c.Param("ueId")
		if ueId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing ueId parameter"})
			return
		}

		subscriber, err := dbInstance.GetSubscriberByUeID(ueId)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscriber not found"})
			return
		}

		c.JSON(http.StatusOK, nmsModels.SubsData{
			PlmnID:          subscriber.PlmnID,
			Sst:             subscriber.Sst,
			Sd:              subscriber.Sd,
			Dnn:             subscriber.Dnn,
			UeId:            subscriber.UeId,
			Opc:             subscriber.OpcValue,
			SequenceNumber:  subscriber.SequenceNumber,
			Key:             subscriber.PermanentKeyValue,
			Var5qi:          subscriber.Var5qi,
			PriorityLevel:   subscriber.PriorityLevel,
			BitrateDownlink: subscriber.BitRateDownlink,
			BitrateUplink:   subscriber.BitRateUplink,
		})
	}
}

func PostSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var subsOverrideData nmsModels.SubsOverrideData
		if err := c.ShouldBindJSON(&subsOverrideData); err != nil {
			logger.NmsLog.Errorln("Invalid input:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}

		ueId := c.Param("ueId")
		if ueId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing ueId parameter"})
			return
		}
		_, err := dbInstance.GetSubscriberByUeID(ueId)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Subscriber already exists"})
			return
		}
		newSubscriber := &db.Subscriber{
			UeId:              ueId,
			PlmnID:            subsOverrideData.PlmnID,
			SequenceNumber:    subsOverrideData.SequenceNumber,
			PermanentKeyValue: subsOverrideData.Key,
			OpcValue:          subsOverrideData.OPc,
		}

		if err := dbInstance.CreateSubscriber(newSubscriber); err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscriber"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Subscriber created successfully"})
	}
}

func DeleteSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		ueId := c.Param("ueId")
		if ueId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing ueId parameter"})
			return
		}

		subscriber, err := dbInstance.GetSubscriberByUeID(ueId)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscriber not found"})
			return
		}

		if err := dbInstance.DeleteSubscriber(subscriber.ID); err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subscriber"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Subscriber deleted successfully"})
	}
}
