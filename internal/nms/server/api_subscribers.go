package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi/models"
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

func convertDbAuthSubsDataToModel(dbAuthSubsData *dbModels.AuthenticationSubscription) models.AuthenticationSubscription {
	if dbAuthSubsData == nil {
		return models.AuthenticationSubscription{}
	}
	authSubsData := models.AuthenticationSubscription{}
	authSubsData.AuthenticationManagementField = dbAuthSubsData.AuthenticationManagementField
	authSubsData.AuthenticationMethod = models.AuthMethod(dbAuthSubsData.AuthenticationMethod)
	if dbAuthSubsData.Milenage != nil {
		authSubsData.Milenage = &models.Milenage{
			Op: &models.Op{
				EncryptionAlgorithm: dbAuthSubsData.Milenage.Op.EncryptionAlgorithm,
				EncryptionKey:       dbAuthSubsData.Milenage.Op.EncryptionKey,
				OpValue:             dbAuthSubsData.Milenage.Op.OpValue,
			},
		}
	}
	if dbAuthSubsData.Opc != nil {
		authSubsData.Opc = &models.Opc{
			EncryptionAlgorithm: dbAuthSubsData.Opc.EncryptionAlgorithm,
			EncryptionKey:       dbAuthSubsData.Opc.EncryptionKey,
			OpcValue:            dbAuthSubsData.Opc.OpcValue,
		}
	}
	if dbAuthSubsData.PermanentKey != nil {
		authSubsData.PermanentKey = &models.PermanentKey{
			EncryptionAlgorithm: dbAuthSubsData.PermanentKey.EncryptionAlgorithm,
			EncryptionKey:       dbAuthSubsData.PermanentKey.EncryptionKey,
			PermanentKeyValue:   dbAuthSubsData.PermanentKey.PermanentKeyValue,
		}
	}
	authSubsData.SequenceNumber = dbAuthSubsData.SequenceNumber

	return authSubsData
}

func convertDbAmDataToModel(dbAmData *dbModels.AccessAndMobilitySubscriptionData) models.AccessAndMobilitySubscriptionData {
	if dbAmData == nil {
		return models.AccessAndMobilitySubscriptionData{}
	}
	amData := models.AccessAndMobilitySubscriptionData{
		Nssai: &models.Nssai{
			DefaultSingleNssais: make([]models.Snssai, 0),
			SingleNssais:        make([]models.Snssai, 0),
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: dbAmData.SubscribedUeAmbr.Downlink,
			Uplink:   dbAmData.SubscribedUeAmbr.Uplink,
		},
	}
	return amData
}

func convertDbSmDataToModel(dbSmData []*dbModels.SessionManagementSubscriptionData) []models.SessionManagementSubscriptionData {
	if dbSmData == nil {
		return nil
	}
	smData := make([]models.SessionManagementSubscriptionData, 0)
	for _, smDataObj := range dbSmData {
		smDataObjModel := models.SessionManagementSubscriptionData{
			DnnConfigurations: make(map[string]models.DnnConfiguration),
		}
		for dnn, dnnConfig := range smDataObj.DnnConfigurations {
			smDataObjModel.DnnConfigurations[dnn] = models.DnnConfiguration{
				PduSessionTypes: &models.PduSessionTypes{
					DefaultSessionType:  models.PduSessionType(dnnConfig.PduSessionTypes.DefaultSessionType),
					AllowedSessionTypes: make([]models.PduSessionType, 0),
				},
				SscModes: &models.SscModes{
					DefaultSscMode:  models.SscMode(dnnConfig.SscModes.DefaultSscMode),
					AllowedSscModes: make([]models.SscMode, 0),
				},
				SessionAmbr: &models.Ambr{
					Downlink: dnnConfig.SessionAmbr.Downlink,
					Uplink:   dnnConfig.SessionAmbr.Uplink,
				},
				Var5gQosProfile: &models.SubscribedDefaultQos{
					Var5qi:        dnnConfig.Var5gQosProfile.Var5qi,
					Arp:           &models.Arp{PriorityLevel: dnnConfig.Var5gQosProfile.Arp.PriorityLevel},
					PriorityLevel: dnnConfig.Var5gQosProfile.PriorityLevel,
				},
			}
			for _, sessionType := range dnnConfig.PduSessionTypes.AllowedSessionTypes {
				smDataObjModel.DnnConfigurations[dnn].PduSessionTypes.AllowedSessionTypes = append(smDataObjModel.DnnConfigurations[dnn].PduSessionTypes.AllowedSessionTypes, models.PduSessionType(sessionType))
			}
			for _, sscMode := range dnnConfig.SscModes.AllowedSscModes {
				smDataObjModel.DnnConfigurations[dnn].SscModes.AllowedSscModes = append(smDataObjModel.DnnConfigurations[dnn].SscModes.AllowedSscModes, models.SscMode(sscMode))
			}
		}
		smData = append(smData, smDataObjModel)
	}
	return smData
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

	authSubsData := convertDbAuthSubsDataToModel(subscriber.AuthenticationSubscription)
	amData := convertDbAmDataToModel(subscriber.AccessAndMobilitySubscriptionData)
	smData := convertDbSmDataToModel(subscriber.SessionManagementSubscriptionData)
	subsData := nmsModels.SubsData{
		Sst:                               subscriber.Sst,
		Sd:                                subscriber.Sd,
		Dnn:                               subscriber.Dnn,
		UeId:                              ueId,
		AuthenticationSubscription:        authSubsData,
		AccessAndMobilitySubscriptionData: amData,
		SessionManagementSubscriptionData: smData,
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

	authSubsData := models.AuthenticationSubscription{
		AuthenticationManagementField: "8000",
		AuthenticationMethod:          "5G_AKA",
		Milenage: &models.Milenage{
			Op: &models.Op{
				EncryptionAlgorithm: 0,
				EncryptionKey:       0,
				OpValue:             "",
			},
		},
		Opc: &models.Opc{
			EncryptionAlgorithm: 0,
			EncryptionKey:       0,
		},
		PermanentKey: &models.PermanentKey{
			EncryptionAlgorithm: 0,
			EncryptionKey:       0,
		},
	}

	if subsOverrideData.OPc != "" {
		authSubsData.Opc.OpcValue = subsOverrideData.OPc
	}
	if subsOverrideData.Key != "" {
		authSubsData.PermanentKey.PermanentKeyValue = subsOverrideData.Key
	}
	if subsOverrideData.SequenceNumber != "" {
		authSubsData.SequenceNumber = subsOverrideData.SequenceNumber
	}
	c.JSON(http.StatusCreated, gin.H{})

	amData := dbModels.AccessAndMobilitySubscriptionData{}

	imsiVal := strings.ReplaceAll(ueId, "imsi-", "")
	imsiData[imsiVal] = &authSubsData
	dbAuthSubsData := dbModels.AuthenticationSubscription{
		AuthenticationManagementField: authSubsData.AuthenticationManagementField,
		AuthenticationMethod:          dbModels.AuthMethod(authSubsData.AuthenticationMethod),
		Milenage: &dbModels.Milenage{
			Op: &dbModels.Op{
				EncryptionAlgorithm: authSubsData.Milenage.Op.EncryptionAlgorithm,
				EncryptionKey:       authSubsData.Milenage.Op.EncryptionKey,
				OpValue:             authSubsData.Milenage.Op.OpValue,
			},
		},
		Opc: &dbModels.Opc{
			EncryptionAlgorithm: authSubsData.Opc.EncryptionAlgorithm,
			EncryptionKey:       authSubsData.Opc.EncryptionKey,
			OpcValue:            authSubsData.Opc.OpcValue,
		},
		PermanentKey: &dbModels.PermanentKey{
			EncryptionAlgorithm: authSubsData.PermanentKey.EncryptionAlgorithm,
			EncryptionKey:       authSubsData.PermanentKey.EncryptionKey,
			PermanentKeyValue:   authSubsData.PermanentKey.PermanentKeyValue,
		},
		SequenceNumber: authSubsData.SequenceNumber,
	}

	dbSubscriber := &dbModels.Subscriber{
		UeId:                              ueId,
		AuthenticationSubscription:        &dbAuthSubsData,
		AccessAndMobilitySubscriptionData: &amData,
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

	imsiVal := strings.ReplaceAll(ueId, "imsi-", "")
	imsiData[imsiVal] = &subsData.AuthenticationSubscription
	subscriber, err := queries.GetSubscriber(ueId)
	if err != nil {
		logger.NmsLog.Warnln(err)
		return
	}
	amData := &dbModels.AccessAndMobilitySubscriptionData{}
	subscriber.AccessAndMobilitySubscriptionData = amData
	subscriber.AuthenticationSubscription = &dbModels.AuthenticationSubscription{
		AuthenticationManagementField: subsData.AuthenticationSubscription.AuthenticationManagementField,
		AuthenticationMethod:          dbModels.AuthMethod(subsData.AuthenticationSubscription.AuthenticationMethod),
		Milenage: &dbModels.Milenage{
			Op: &dbModels.Op{
				EncryptionAlgorithm: subsData.AuthenticationSubscription.Milenage.Op.EncryptionAlgorithm,
				EncryptionKey:       subsData.AuthenticationSubscription.Milenage.Op.EncryptionKey,
				OpValue:             subsData.AuthenticationSubscription.Milenage.Op.OpValue,
			},
		},
		Opc: &dbModels.Opc{
			EncryptionAlgorithm: subsData.AuthenticationSubscription.Opc.EncryptionAlgorithm,
			EncryptionKey:       subsData.AuthenticationSubscription.Opc.EncryptionKey,
			OpcValue:            subsData.AuthenticationSubscription.Opc.OpcValue,
		},
		PermanentKey: &dbModels.PermanentKey{
			EncryptionAlgorithm: subsData.AuthenticationSubscription.PermanentKey.EncryptionAlgorithm,
			EncryptionKey:       subsData.AuthenticationSubscription.PermanentKey.EncryptionKey,
			PermanentKeyValue:   subsData.AuthenticationSubscription.PermanentKey.PermanentKeyValue,
		},
		SequenceNumber: subsData.AuthenticationSubscription.SequenceNumber,
	}

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
