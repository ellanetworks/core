package server

import (
	"encoding/hex"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type CreateSubscriberParams struct {
	Imsi           string `json:"imsi"`
	Key            string `json:"key"`
	Opc            string `json:"opc,omitempty"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profileName"`
}

type UpdateSubscriberParams struct {
	Imsi        string `json:"imsi"`
	ProfileName string `json:"profileName"`
}

type GetSubscriberResponse struct {
	Imsi           string `json:"imsi"`
	IPAddress      string `json:"ipAddress"`
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
	network, err := dbInstance.GetOperator()
	if err != nil {
		logger.APILog.Warn("Failed to retrieve operator", zap.Error(err))
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
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		tracer := otel.Tracer("ella-core/api")
		_, span := tracer.Start(c.Request.Context(), "db.ListSubscribers")
		dbSubscribers, err := dbInstance.ListSubscribers(c.Request.Context())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "ListSubscribers failed")
			span.End()
			writeError(c, http.StatusInternalServerError, "Unable to retrieve subscribers")
			return
		}
		span.End()

		subscribers := make([]GetSubscriberResponse, 0)
		for _, dbSubscriber := range dbSubscribers {
			profile, err := dbInstance.GetProfileByID(dbSubscriber.ProfileID)
			if err != nil {
				writeError(c, http.StatusInternalServerError, "Failed to retrieve profile")
				return
			}
			subscribers = append(subscribers, GetSubscriberResponse{
				Imsi:           dbSubscriber.Imsi,
				IPAddress:      dbSubscriber.IPAddress,
				Opc:            dbSubscriber.Opc,
				Key:            dbSubscriber.PermanentKey,
				SequenceNumber: dbSubscriber.SequenceNumber,
				ProfileName:    profile.Name,
			})
		}
		writeResponse(c, subscribers, http.StatusOK)
		logger.LogAuditEvent(
			ListSubscribersAction,
			email,
			c.ClientIP(),
			"User listed subscribers",
		)
	}
}

func GetSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		imsi := c.Param("imsi")
		if imsi == "" {
			writeError(c, http.StatusBadRequest, "Missing imsi parameter")
			return
		}

		dbSubscriber, err := dbInstance.GetSubscriber(imsi, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Subscriber not found")
			return
		}
		profile, err := dbInstance.GetProfileByID(dbSubscriber.ProfileID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to retrieve profile")
			return
		}

		subscriber := GetSubscriberResponse{
			Imsi:           dbSubscriber.Imsi,
			IPAddress:      dbSubscriber.IPAddress,
			Opc:            dbSubscriber.Opc,
			SequenceNumber: dbSubscriber.SequenceNumber,
			Key:            dbSubscriber.PermanentKey,
			ProfileName:    profile.Name,
		}
		writeResponse(c, subscriber, http.StatusOK)
		logger.LogAuditEvent(
			GetSubscriberAction,
			email,
			c.ClientIP(),
			"User retrieved subscriber: "+imsi,
		)
	}
}

func CreateSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var createSubscriberParams CreateSubscriberParams
		err := c.ShouldBindJSON(&createSubscriberParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createSubscriberParams.Imsi == "" {
			writeError(c, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		if createSubscriberParams.SequenceNumber == "" {
			writeError(c, http.StatusBadRequest, "Missing sequenceNumber parameter")
			return
		}
		if createSubscriberParams.Key == "" {
			writeError(c, http.StatusBadRequest, "Missing key parameter")
			return
		}
		if createSubscriberParams.ProfileName == "" {
			writeError(c, http.StatusBadRequest, "Missing profileName parameter")
			return
		}
		if !isImsiValid(createSubscriberParams.Imsi, dbInstance) {
			writeError(c, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.")
			return
		}
		if !isSequenceNumberValid(createSubscriberParams.SequenceNumber) {
			writeError(c, http.StatusBadRequest, "Invalid sequenceNumber. Must be a 6-byte hexadecimal string.")
			return
		}
		if !isHexString(createSubscriberParams.Key) {
			writeError(c, http.StatusBadRequest, "Invalid key format. Must be a 32-character hexadecimal string.")
			return
		}
		if createSubscriberParams.Opc != "" {
			if !isHexString(createSubscriberParams.Opc) {
				writeError(c, http.StatusBadRequest, "Invalid OPc format. Must be a 32-character hexadecimal string.")
				return
			}
		}

		K, err := hex.DecodeString(createSubscriberParams.Key)
		if err != nil {
			logger.APILog.Warn("Failed to decode key", zap.Error(err))
			writeError(c, http.StatusBadRequest, "Invalid key format")
			return
		}

		var opcHex string
		if createSubscriberParams.Opc == "" {
			opCodeHex, err := dbInstance.GetOperatorCode()
			if err != nil {
				logger.APILog.Warn("Failed to retrieve operator code", zap.Error(err))
				writeError(c, http.StatusInternalServerError, "Failed to retrieve operator code")
				return
			}
			OP, err := hex.DecodeString(opCodeHex)
			if err != nil {
				logger.APILog.Warn("Failed to decode OP", zap.Error(err))
				writeError(c, http.StatusInternalServerError, "Failed to decode OP")
				return
			}

			opc, err := deriveOPc(K, OP)
			if err != nil {
				logger.APILog.Warn("Failed to derive OPc", zap.Error(err))
				writeError(c, http.StatusInternalServerError, "Failed to generate OPc")
				return
			}
			opcHex = hex.EncodeToString(opc)
		} else {
			opcHex = createSubscriberParams.Opc
		}

		_, err = dbInstance.GetSubscriber(createSubscriberParams.Imsi, c.Request.Context())
		if err == nil {
			writeError(c, http.StatusBadRequest, "Subscriber already exists")
			return
		}
		profile, err := dbInstance.GetProfile(createSubscriberParams.ProfileName)
		if err != nil {
			writeError(c, http.StatusNotFound, "Profile not found")
			return
		}
		newSubscriber := &db.Subscriber{
			Imsi:           createSubscriberParams.Imsi,
			SequenceNumber: createSubscriberParams.SequenceNumber,
			PermanentKey:   createSubscriberParams.Key,
			Opc:            opcHex,
			ProfileID:      profile.ID,
		}

		if err := dbInstance.CreateSubscriber(newSubscriber, c.Request.Context()); err != nil {
			logger.APILog.Warn("Failed to create subscriber", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to create subscriber")
			return
		}

		response := SuccessResponse{Message: "Subscriber created successfully"}
		writeResponse(c, response, http.StatusCreated)
		logger.LogAuditEvent(
			CreateSubscriberAction,
			email,
			c.ClientIP(),
			"User created subscriber: "+createSubscriberParams.Imsi,
		)
	}
}

func UpdateSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		imsi := c.Param("imsi")
		if imsi == "" {
			writeError(c, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		var updateSubscriberParams UpdateSubscriberParams
		err := c.ShouldBindJSON(&updateSubscriberParams)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if updateSubscriberParams.Imsi == "" {
			writeError(c, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		if updateSubscriberParams.ProfileName == "" {
			writeError(c, http.StatusBadRequest, "Missing profileName parameter")
			return
		}
		if !isImsiValid(updateSubscriberParams.Imsi, dbInstance) {
			writeError(c, http.StatusBadRequest, "Invalid IMSI format. Must be a 15-digit string starting with `<mcc><mnc>`.")
			return
		}

		existingSubscriber, err := dbInstance.GetSubscriber(imsi, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Subscriber not found")
			return
		}
		profile, err := dbInstance.GetProfile(updateSubscriberParams.ProfileName)
		if err != nil {
			writeError(c, http.StatusNotFound, "Profile not found")
			return
		}
		updatedSubscriber := &db.Subscriber{
			Imsi:           existingSubscriber.Imsi,
			SequenceNumber: existingSubscriber.SequenceNumber,
			PermanentKey:   existingSubscriber.PermanentKey,
			Opc:            existingSubscriber.Opc,
			ProfileID:      profile.ID,
		}

		if err := dbInstance.UpdateSubscriber(updatedSubscriber, c.Request.Context()); err != nil {
			logger.APILog.Warn("Failed to update subscriber", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to update subscriber")
			return
		}

		response := SuccessResponse{Message: "Subscriber updated successfully"}
		writeResponse(c, response, http.StatusOK)
		logger.LogAuditEvent(
			UpdateSubscriberAction,
			email,
			c.ClientIP(),
			"User updated subscriber: "+imsi,
		)
	}
}

func DeleteSubscriber(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		imsi := c.Param("imsi")
		if imsi == "" {
			writeError(c, http.StatusBadRequest, "Missing imsi parameter")
			return
		}
		_, err := dbInstance.GetSubscriber(imsi, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Subscriber not found")
			return
		}
		err = dbInstance.DeleteSubscriber(imsi, c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to delete subscriber", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Failed to delete subscriber")
			return
		}

		response := SuccessResponse{Message: "Subscriber deleted successfully"}
		writeResponse(c, response, http.StatusOK)
		logger.LogAuditEvent(
			DeleteSubscriberAction,
			email,
			c.ClientIP(),
			"User deleted subscriber: "+imsi,
		)
	}
}
