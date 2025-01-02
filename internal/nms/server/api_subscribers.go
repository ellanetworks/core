package server

import (
	"encoding/hex"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
)

type CreateSubscriberParams struct {
	Imsi           string `json:"imsi"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profileName"`
}

type UpdateSubscriberParams struct {
	Imsi        string `json:"imsi"`
	ProfileName string `json:"profileName"`
}

type GetSubscriberResponse struct {
	Imsi           string `json:"imsi"`
	IpAddress      string `json:"ipAddress"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
	Key            string `json:"key"`
	ProfileName    string `json:"profileName"`
}

const (
	ListSubscribersAction  = "list_subscribers"
	GetSubscriberAction    = "get_subscriber"
	CreateSubscriberAction = "create_subscriber"
	UpdateSubscriberAction = "update_subscriber"
	DeleteSubscriberAction = "delete_subscriber"
)

func isImsiValid(imsi string, dbInstance *db.Database) bool {
	if len(imsi) != 15 {
		return false
	}
	network, err := dbInstance.GetOperatorId()
	if err != nil {
		logger.NmsLog.Warnf("Failed to retrieve network: %v", err)
		return false
	}
	Mcc := network.Mcc
	Mnc := network.Mnc
	if imsi[:3] != Mcc || imsi[3:5] != Mnc {
		return false
	}
	return true
}

func isHexString(input string) bool {
	_, err := hex.DecodeString(input)
	return err == nil
}

func isSequenceNumberValid(sequenceNumber string) bool {
	bytes, err := hex.DecodeString(sequenceNumber)
	if err != nil {
		return false
	}
	return len(bytes) == 6
}

func ListSubscribers(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		dbSubscribers, err := dbInstance.ListSubscribers()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve subscribers")
			return
		}

		subscribers := make([]GetSubscriberResponse, 0)
		for _, dbSubscriber := range dbSubscribers {
			profile, err := dbInstance.GetProfileByID(dbSubscriber.ProfileID)
			if err != nil {
				writeError(c.Writer, http.StatusInternalServerError, "Failed to retrieve profile")
				return
			}
			subscribers = append(subscribers, GetSubscriberResponse{
				Imsi:           dbSubscriber.Imsi,
				IpAddress:      dbSubscriber.IpAddress,
				Opc:            dbSubscriber.Opc,
				Key:            dbSubscriber.PermanentKey,
				SequenceNumber: dbSubscriber.SequenceNumber,
				ProfileName:    profile.Name,
			})
		}
		err = writeResponse(c.Writer, subscribers, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			ListSubscribersAction,
			username,
			"User listed subscribers",
		)
	}
}

func GetSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		imsi := c.Param("imsi")
		if imsi == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing imsi parameter")
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(imsi)
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
			Imsi:           dbSubscriber.Imsi,
			IpAddress:      dbSubscriber.IpAddress,
			Opc:            dbSubscriber.Opc,
			SequenceNumber: dbSubscriber.SequenceNumber,
			Key:            dbSubscriber.PermanentKey,
			ProfileName:    profile.Name,
		}
		err = writeResponse(c.Writer, subscriber, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			GetSubscriberAction,
			username,
			"User retrieved subscriber: "+imsi,
		)
	}
}

func CreateSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		var createSubscriberParams CreateSubscriberParams
		err := c.ShouldBindJSON(&createSubscriberParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createSubscriberParams.Imsi == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing imsi parameter")
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
		if createSubscriberParams.ProfileName == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing profileName parameter")
			return
		}
		if !isImsiValid(createSubscriberParams.Imsi, dbInstance) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.")
			return
		}
		if !isSequenceNumberValid(createSubscriberParams.SequenceNumber) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.")
			return
		}
		if !isHexString(createSubscriberParams.Key) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid key format. Must be a 32-character hexadecimal string.")
			return
		}

		K, err := hex.DecodeString(createSubscriberParams.Key)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusBadRequest, "Invalid key format")
			return
		}
		opCodeHex, err := dbInstance.GetOperatorCode()
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to retrieve operator code")
			return
		}
		OP, err := hex.DecodeString(opCodeHex)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to decode OP")
			return
		}

		opc, err := deriveOPc(K, OP)
		if err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to generate OPc")
			return
		}
		opcHex := hex.EncodeToString(opc)

		_, err = dbInstance.GetSubscriber(createSubscriberParams.Imsi)
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
			Imsi:           createSubscriberParams.Imsi,
			SequenceNumber: createSubscriberParams.SequenceNumber,
			PermanentKey:   createSubscriberParams.Key,
			Opc:            opcHex,
			ProfileID:      profile.ID,
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
		logger.LogAuditEvent(
			CreateSubscriberAction,
			username,
			"User created subscriber: "+createSubscriberParams.Imsi,
		)
	}
}

func UpdateSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		imsi := c.Param("imsi")
		if imsi == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		var updateSubscriberParams UpdateSubscriberParams
		err := c.ShouldBindJSON(&updateSubscriberParams)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateSubscriberParams.Imsi == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		if updateSubscriberParams.ProfileName == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing profileName parameter")
			return
		}
		if !isImsiValid(updateSubscriberParams.Imsi, dbInstance) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.")
			return
		}

		existingSubscriber, err := dbInstance.GetSubscriber(imsi)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Subscriber not found")
			return
		}
		profile, err := dbInstance.GetProfile(updateSubscriberParams.ProfileName)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Profile not found")
			return
		}
		updatedSubscriber := &db.Subscriber{
			Imsi:           existingSubscriber.Imsi,
			SequenceNumber: existingSubscriber.SequenceNumber,
			PermanentKey:   existingSubscriber.PermanentKey,
			Opc:            existingSubscriber.Opc,
			ProfileID:      profile.ID,
		}

		if err := dbInstance.UpdateSubscriber(updatedSubscriber); err != nil {
			logger.NmsLog.Warnln(err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to update subscriber")
			return
		}

		response := SuccessResponse{Message: "Subscriber updated successfully"}
		err = writeResponse(c.Writer, response, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
		logger.LogAuditEvent(
			UpdateSubscriberAction,
			username,
			"User updated subscriber: "+imsi,
		)
	}
}

func DeleteSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		usernameAny, _ := c.Get("username")
		username, ok := usernameAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get username"})
			return
		}
		imsi := c.Param("imsi")
		if imsi == "" {
			writeError(c.Writer, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		_, err := dbInstance.GetSubscriber(imsi)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Subscriber not found")
			return
		}
		err = dbInstance.DeleteSubscriber(imsi)
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
		logger.LogAuditEvent(
			DeleteSubscriberAction,
			username,
			"User deleted subscriber: "+imsi,
		)
	}
}
