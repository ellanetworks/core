package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
)

type CreateSubscriberParams struct {
	UeId           string `json:"ueId"`
	OPc            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profileName"`
}

type GetSubscriberResponse struct {
	UeId           string `json:"ueId"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
	Key            string `json:"key"`
	ProfileName    string `json:"profileName"`
}

func ListSubscribers(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		dbSubscribers, err := dbInstance.ListSubscribers()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve subscribers")
			return
		}

		subscribers := make([]GetSubscriberResponse, 0)
		for _, dbSubscriber := range dbSubscribers {
			subscribers = append(subscribers, GetSubscriberResponse{
				UeId: dbSubscriber.UeId,
			})
		}
		err = writeResponse(c.Writer, subscribers, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func GetSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		ueId := c.Param("ueId")
		if ueId == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing ueId parameter")
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(ueId)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Subscriber not found")
			return
		}
		profile, err := dbInstance.GetProfileByID(dbSubscriber.ProfileID)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to retrieve profile")
			return
		}

		subscriber := GetSubscriberResponse{
			UeId:           dbSubscriber.UeId,
			Opc:            dbSubscriber.OpcValue,
			SequenceNumber: dbSubscriber.SequenceNumber,
			Key:            dbSubscriber.PermanentKeyValue,
			ProfileName:    profile.Name,
		}
		err = writeResponse(c.Writer, subscriber, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func CreateSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		var createSubscriberParams CreateSubscriberParams
		err := c.ShouldBindJSON(&createSubscriberParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createSubscriberParams.UeId == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing ueId parameter")
			return
		}
		if createSubscriberParams.SequenceNumber == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing sequenceNumber parameter")
			return
		}
		if createSubscriberParams.Key == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing key parameter")
			return
		}
		if createSubscriberParams.OPc == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing opc parameter")
			return
		}
		if createSubscriberParams.ProfileName == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing profileName parameter")
			return
		}

		_, err = dbInstance.GetSubscriber(createSubscriberParams.UeId)
		if err == nil {
			writeError(c.Writer, http.StatusBadRequest, "Subscriber already exists")
			return
		}
		profile, err := dbInstance.GetProfile(createSubscriberParams.ProfileName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}
		newSubscriber := &db.Subscriber{
			UeId:              createSubscriberParams.UeId,
			SequenceNumber:    createSubscriberParams.SequenceNumber,
			PermanentKeyValue: createSubscriberParams.Key,
			OpcValue:          createSubscriberParams.OPc,
			ProfileID:         profile.ID,
		}

		if err := dbInstance.CreateSubscriber(newSubscriber); err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create subscriber")
			return
		}

		response := SuccessResponse{Message: "Subscriber created successfully"}
		err = writeResponse(c.Writer, response, http.StatusCreated)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}

func DeleteSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		ueId := c.Param("ueId")
		if ueId == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing ueId parameter")
			return
		}
		_, err := dbInstance.GetSubscriber(ueId)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Subscriber not found")
			return
		}
		err = dbInstance.DeleteSubscriber(ueId)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to delete subscriber")
			return
		}

		response := SuccessResponse{Message: "Subscriber deleted successfully"}
		err = writeResponse(c.Writer, response, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
