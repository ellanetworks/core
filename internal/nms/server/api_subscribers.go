package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/logger"
	nmsModels "github.com/yeastengine/ella/internal/nms/models"
)

func setCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
}

func GetSubscribers(c *gin.Context) {
	setCorsHeader(c)
	var subsList []nmsModels.SubsListIE
	subscribers, err := queries.ListSubscribers()
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	for _, subscriber := range subscribers {
		subscriber := nmsModels.SubsListIE{
			PlmnID: subscriber.UeId,
			UeId:   subscriber.UeId,
		}
		subsList = append(subsList, subscriber)
	}
	c.JSON(http.StatusOK, subsList)
}

func GetSubscriberByID(c *gin.Context) {
	setCorsHeader(c)

	logger.NmsLog.Infoln("Get One Subscriber Data")

	ueId := c.Param("ueId")
	if ueId == "" {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}

	subsData := nmsModels.SubsData{
		Sst:             subscriber.Sst,
		Sd:              subscriber.Sd,
		Dnn:             subscriber.Dnn,
		UeId:            ueId,
		Opc:             subscriber.OpcValue,
		SequenceNumber:  subscriber.SequenceNumber,
		Key:             subscriber.PermanentKeyValue,
		Var5qi:          subscriber.Var5qi,
		PriorityLevel:   subscriber.PriorityLevel,
		BitrateDownlink: subscriber.BitRateDownlink,
		BitrateUplink:   subscriber.BitRateUplink,
	}

	c.JSON(http.StatusOK, subsData)
}

func PostSubscriberByID(c *gin.Context) {
	setCorsHeader(c)

	var subsOverrideData nmsModels.SubsOverrideData
	if err := c.ShouldBindJSON(&subsOverrideData); err != nil {
		logger.NmsLog.Errorln("Post One Subscriber Data - ShouldBindJSON failed ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ueId := c.Param("ueId")

	c.JSON(http.StatusCreated, gin.H{})

	dbSubscriber := &dbModels.Subscriber{
		UeId:              ueId,
		OpcValue:          subsOverrideData.OPc,
		PermanentKeyValue: subsOverrideData.Key,
		SequenceNumber:    subsOverrideData.SequenceNumber,
		BitRateUplink:     "",
		BitRateDownlink:   "",
	}
	err := queries.CreateSubscriber(dbSubscriber)
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	logger.NmsLog.Infof("Created subscriber: %v", ueId)
}

func PutSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.NmsLog.Infoln("Put One Subscriber Data")

	var subsData nmsModels.SubsData
	if err := c.ShouldBindJSON(&subsData); err != nil {
		logger.NmsLog.Panic(err.Error())
	}

	ueId := c.Param("ueId")
	c.JSON(http.StatusNoContent, gin.H{})

	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}

	subscriber.OpcValue = subsData.Opc
	subscriber.SequenceNumber = subsData.SequenceNumber
	subscriber.PermanentKeyValue = subsData.Key

	err = queries.CreateSubscriber(subscriber)
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}

	logger.NmsLog.Infof("Edited Subscriber: %v", ueId)
}

func PatchSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.NmsLog.Infoln("Patch One Subscriber Data")
}

func DeleteSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	ueId := c.Param("ueId")
	c.JSON(http.StatusNoContent, gin.H{})
	err := queries.DeleteSubscriber(ueId)
	if err != nil {
		logger.NmsLog.Warnln(err)
	}
	logger.NmsLog.Infof("Deleted Subscriber: %v", ueId)
}
