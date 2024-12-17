package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
)

type CreateSubscriberParams struct {
	UeId           string `json:"ueId"`
	PlmnID         string `json:"plmnID"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
}

type GetSubscriberResponse struct {
	UeId            string `json:"ueId"`
	PlmnID          string `json:"plmnID"`
	Sst             int32  `json:"sst" yaml:"sst" bson:"sst" mapstructure:"Sst"`
	Sd              string `json:"sd,omitempty" yaml:"sd" bson:"sd" mapstructure:"Sd"`
	Dnn             string `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`
	Opc             string `json:"opc"`
	SequenceNumber  string `json:"sequenceNumber"`
	Key             string `json:"key"`
	BitrateDownlink string `json:"bitrateDownlink"`
	BitrateUplink   string `json:"bitrateUplink"`
	Var5qi          int32  `json:"var5qi"`
	PriorityLevel   int32  `json:"priorityLevel"`
}

func ListSubscribers(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		subscribers, err := dbInstance.ListSubscribers()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve subscribers"})
			return
		}

		var subsList []GetSubscriberResponse
		for _, subscriber := range subscribers {
			subsList = append(subsList, GetSubscriberResponse{
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

		subscriber, err := dbInstance.GetSubscriber(ueId)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscriber not found"})
			return
		}

		c.JSON(http.StatusOK, GetSubscriberResponse{
			UeId:            subscriber.UeId,
			PlmnID:          subscriber.PlmnID,
			Sst:             subscriber.Sst,
			Sd:              subscriber.Sd,
			Dnn:             subscriber.Dnn,
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

func CreateSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var createSubscriberParams CreateSubscriberParams
		if err := c.ShouldBindJSON(&createSubscriberParams); err != nil {
			logger.NmsLog.Errorln("Invalid input:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
		}
		if createSubscriberParams.UeId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing ueId parameter"})
			return
		}
		if createSubscriberParams.PlmnID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing plmnID parameter"})
			return
		}
		if createSubscriberParams.SequenceNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing sequenceNumber parameter"})
			return
		}
		if createSubscriberParams.Key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing key parameter"})
			return
		}
		if createSubscriberParams.OPc == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing opc parameter"})
			return
		}

		_, err := dbInstance.GetSubscriber(createSubscriberParams.UeId)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Subscriber already exists"})
			return
		}
		newSubscriber := &db.Subscriber{
			UeId:              createSubscriberParams.UeId,
			PlmnID:            createSubscriberParams.PlmnID,
			SequenceNumber:    createSubscriberParams.SequenceNumber,
			PermanentKeyValue: createSubscriberParams.Key,
			OpcValue:          createSubscriberParams.OPc,
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
		_, err := dbInstance.GetSubscriber(ueId)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscriber not found"})
			return
		}
		err = dbInstance.DeleteSubscriber(ueId)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subscriber"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Subscriber deleted successfully"})
	}
}
