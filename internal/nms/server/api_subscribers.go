package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi/models"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/nms/logger"
	nmsModels "github.com/yeastengine/ella/internal/nms/models"
)

func setCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
}

func GetSampleJSON(c *gin.Context) {
	setCorsHeader(c)

	logger.NMSLog.Infoln("Get a JSON Example")

	var subsData nmsModels.SubsData

	authSubsData := models.AuthenticationSubscription{
		AuthenticationManagementField: "8000",
		AuthenticationMethod:          "5G_AKA", // "5G_AKA", "EAP_AKA_PRIME"
		Milenage: &models.Milenage{
			Op: &models.Op{
				EncryptionAlgorithm: 0,
				EncryptionKey:       0,
				OpValue:             "c9e8763286b5b9ffbdf56e1297d0887b", // Required
			},
		},
		Opc: &models.Opc{
			EncryptionAlgorithm: 0,
			EncryptionKey:       0,
			OpcValue:            "981d464c7c52eb6e5036234984ad0bcf", // Required
		},
		PermanentKey: &models.PermanentKey{
			EncryptionAlgorithm: 0,
			EncryptionKey:       0,
			PermanentKeyValue:   "5122250214c33e723a5dd523fc145fc0", // Required
		},
		SequenceNumber: "16f3b3f70fc2",
	}

	amDataData := models.AccessAndMobilitySubscriptionData{
		Gpsis: []string{
			"msisdn-0900000000",
		},
		Nssai: &models.Nssai{
			DefaultSingleNssais: []models.Snssai{
				{
					Sd:  "010203",
					Sst: 1,
				},
				{
					Sd:  "112233",
					Sst: 1,
				},
			},
			SingleNssais: []models.Snssai{
				{
					Sd:  "010203",
					Sst: 1,
				},
				{
					Sd:  "112233",
					Sst: 1,
				},
			},
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: "1000 Kbps",
			Uplink:   "1000 Kbps",
		},
	}

	smDataData := []models.SessionManagementSubscriptionData{
		{
			SingleNssai: &models.Snssai{
				Sst: 1,
				Sd:  "010203",
			},
			DnnConfigurations: map[string]models.DnnConfiguration{
				"internet": {
					PduSessionTypes: &models.PduSessionTypes{
						DefaultSessionType:  models.PduSessionType_IPV4,
						AllowedSessionTypes: []models.PduSessionType{models.PduSessionType_IPV4},
					},
					SscModes: &models.SscModes{
						DefaultSscMode:  models.SscMode__1,
						AllowedSscModes: []models.SscMode{models.SscMode__1},
					},
					SessionAmbr: &models.Ambr{
						Downlink: "1000 Kbps",
						Uplink:   "1000 Kbps",
					},
					Var5gQosProfile: &models.SubscribedDefaultQos{
						Var5qi: 9,
						Arp: &models.Arp{
							PriorityLevel: 8,
						},
						PriorityLevel: 8,
					},
				},
			},
		},
		{
			SingleNssai: &models.Snssai{
				Sst: 1,
				Sd:  "112233",
			},
			DnnConfigurations: map[string]models.DnnConfiguration{
				"internet": {
					PduSessionTypes: &models.PduSessionTypes{
						DefaultSessionType:  models.PduSessionType_IPV4,
						AllowedSessionTypes: []models.PduSessionType{models.PduSessionType_IPV4},
					},
					SscModes: &models.SscModes{
						DefaultSscMode:  models.SscMode__1,
						AllowedSscModes: []models.SscMode{models.SscMode__1},
					},
					SessionAmbr: &models.Ambr{
						Downlink: "1000 Kbps",
						Uplink:   "1000 Kbps",
					},
					Var5gQosProfile: &models.SubscribedDefaultQos{
						Var5qi: 9,
						Arp: &models.Arp{
							PriorityLevel: 8,
						},
						PriorityLevel: 8,
					},
				},
			},
		},
	}

	smfSelData := models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: map[string]models.SnssaiInfo{
			"01010203": {
				DnnInfos: []models.DnnInfo{
					{
						Dnn: "internet",
					},
				},
			},
			"01112233": {
				DnnInfos: []models.DnnInfo{
					{
						Dnn: "internet",
					},
				},
			},
		},
	}

	amPolicyData := models.AmPolicyData{
		SubscCats: []string{
			"free5gc",
		},
	}

	smPolicyData := models.SmPolicyData{
		SmPolicySnssaiData: map[string]models.SmPolicySnssaiData{
			"01010203": {
				Snssai: &models.Snssai{
					Sd:  "010203",
					Sst: 1,
				},
				SmPolicyDnnData: map[string]models.SmPolicyDnnData{
					"internet": {
						Dnn: "internet",
					},
				},
			},
			"01112233": {
				Snssai: &models.Snssai{
					Sd:  "112233",
					Sst: 1,
				},
				SmPolicyDnnData: map[string]models.SmPolicyDnnData{
					"internet": {
						Dnn: "internet",
					},
				},
			},
		},
	}

	servingPlmnId := "20893"
	ueId := "imsi-2089300007487"

	subsData = nmsModels.SubsData{
		PlmnID:                            servingPlmnId,
		UeId:                              ueId,
		AuthenticationSubscription:        authSubsData,
		AccessAndMobilitySubscriptionData: amDataData,
		SessionManagementSubscriptionData: smDataData,
		SmfSelectionSubscriptionData:      smfSelData,
		AmPolicyData:                      amPolicyData,
		SmPolicyData:                      smPolicyData,
	}
	c.JSON(http.StatusOK, subsData)
}

func GetSubscribers(c *gin.Context) {
	setCorsHeader(c)
	var subsList []nmsModels.SubsListIE
	amDataList, err := queries.ListAmData()
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	for _, amData := range amDataList {
		subscriber := nmsModels.SubsListIE{
			UeId:   amData.UeId,
			PlmnID: amData.ServingPlmnId,
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
		Gpsis: dbAmData.Gpsis,
		Nssai: &models.Nssai{
			DefaultSingleNssais: make([]models.Snssai, 0),
			SingleNssais:        make([]models.Snssai, 0),
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: dbAmData.SubscribedUeAmbr.Downlink,
			Uplink:   dbAmData.SubscribedUeAmbr.Uplink,
		},
	}
	for _, snssai := range dbAmData.Nssai.DefaultSingleNssais {
		amData.Nssai.DefaultSingleNssais = append(amData.Nssai.DefaultSingleNssais, models.Snssai{
			Sd:  snssai.Sd,
			Sst: snssai.Sst,
		})
	}
	for _, snssai := range dbAmData.Nssai.SingleNssais {
		amData.Nssai.SingleNssais = append(amData.Nssai.SingleNssais, models.Snssai{
			Sd:  snssai.Sd,
			Sst: snssai.Sst,
		})
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
			SingleNssai: &models.Snssai{
				Sst: smDataObj.SingleNssai.Sst,
				Sd:  smDataObj.SingleNssai.Sd,
			},
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

func convertDbSmfSelectionDataToModel(dbSmfSelectionData *dbModels.SmfSelectionSubscriptionData) models.SmfSelectionSubscriptionData {
	if dbSmfSelectionData == nil {
		return models.SmfSelectionSubscriptionData{}
	}
	smfSelectionData := models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: make(map[string]models.SnssaiInfo),
	}
	for snssai, dbSnssaiInfo := range dbSmfSelectionData.SubscribedSnssaiInfos {
		smfSelectionData.SubscribedSnssaiInfos[snssai] = models.SnssaiInfo{
			DnnInfos: make([]models.DnnInfo, 0),
		}
		snssaiInfo := smfSelectionData.SubscribedSnssaiInfos[snssai]
		for _, dbDnnInfo := range dbSnssaiInfo.DnnInfos {
			snssaiInfo.DnnInfos = append(snssaiInfo.DnnInfos, models.DnnInfo{
				Dnn: dbDnnInfo.Dnn,
			})
		}
		smfSelectionData.SubscribedSnssaiInfos[snssai] = snssaiInfo
	}
	return smfSelectionData
}

func convertDbAmPolicyDataToModel(dbAmPolicyData *dbModels.AmPolicyData) models.AmPolicyData {
	if dbAmPolicyData == nil {
		return models.AmPolicyData{}
	}
	amPolicyData := models.AmPolicyData{
		SubscCats: dbAmPolicyData.SubscCats,
	}
	return amPolicyData
}

func convertDbSmPolicyDataToModel(dbSmPolicyData *dbModels.SmPolicyData) models.SmPolicyData {
	if dbSmPolicyData == nil {
		return models.SmPolicyData{}
	}
	smPolicyData := models.SmPolicyData{
		SmPolicySnssaiData: make(map[string]models.SmPolicySnssaiData),
	}
	for snssai, dbSmPolicySnssaiData := range dbSmPolicyData.SmPolicySnssaiData {
		smPolicyData.SmPolicySnssaiData[snssai] = models.SmPolicySnssaiData{
			Snssai: &models.Snssai{
				Sd:  dbSmPolicySnssaiData.Snssai.Sd,
				Sst: dbSmPolicySnssaiData.Snssai.Sst,
			},
			SmPolicyDnnData: make(map[string]models.SmPolicyDnnData),
		}
		smPolicySnssaiData := smPolicyData.SmPolicySnssaiData[snssai]
		for dnn, dbSmPolicyDnnData := range dbSmPolicySnssaiData.SmPolicyDnnData {
			smPolicySnssaiData.SmPolicyDnnData[dnn] = models.SmPolicyDnnData{
				Dnn: dbSmPolicyDnnData.Dnn,
			}
		}
		smPolicyData.SmPolicySnssaiData[snssai] = smPolicySnssaiData
	}
	return smPolicyData
}

func GetSubscriberByID(c *gin.Context) {
	setCorsHeader(c)

	logger.NMSLog.Infoln("Get One Subscriber Data")

	var subsData nmsModels.SubsData

	ueId := c.Param("ueId")

	dbAuthSubsData, err := queries.GetAuthenticationSubscription(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	if dbAuthSubsData == nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	dbAmData, err := queries.GetAmData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	dbSmData, err := queries.ListSmData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	dbSmfSelectionData, err := queries.GetSmfSelectionSubscriptionData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	dbAmPolicyData, err := queries.GetAmPolicyData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	dbSmPolicyData, err := queries.GetSmPolicyData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	authSubsData := convertDbAuthSubsDataToModel(dbAuthSubsData)
	amData := convertDbAmDataToModel(dbAmData)
	smData := convertDbSmDataToModel(dbSmData)
	smfSelectionData := convertDbSmfSelectionDataToModel(dbSmfSelectionData)
	amPolicyData := convertDbAmPolicyDataToModel(dbAmPolicyData)
	smPolicyData := convertDbSmPolicyDataToModel(dbSmPolicyData)
	subsData = nmsModels.SubsData{
		UeId:                              ueId,
		AuthenticationSubscription:        authSubsData,
		AccessAndMobilitySubscriptionData: amData,
		SessionManagementSubscriptionData: smData,
		SmfSelectionSubscriptionData:      smfSelectionData,
		AmPolicyData:                      amPolicyData,
		SmPolicyData:                      smPolicyData,
	}

	c.JSON(http.StatusOK, subsData)
}

func PostSubscriberByID(c *gin.Context) {
	setCorsHeader(c)

	var subsOverrideData nmsModels.SubsOverrideData
	if err := c.ShouldBindJSON(&subsOverrideData); err != nil {
		logger.NMSLog.Errorln("Post One Subscriber Data - ShouldBindJSON failed ", err)
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

	err := queries.CreateAmData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	imsiVal := strings.ReplaceAll(ueId, "imsi-", "")
	imsiData[imsiVal] = &authSubsData
	dbAuthSubsData := &dbModels.AuthenticationSubscription{
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
	err = queries.CreateAuthenticationSubscription(ueId, dbAuthSubsData)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	logger.NMSLog.Infof("Created subscriber: %v", ueId)
}

func PutSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Put One Subscriber Data")

	var subsData nmsModels.SubsData
	if err := c.ShouldBindJSON(&subsData); err != nil {
		logger.NMSLog.Panic(err.Error())
	}

	ueId := c.Param("ueId")
	c.JSON(http.StatusNoContent, gin.H{})

	imsiVal := strings.ReplaceAll(ueId, "imsi-", "")
	imsiData[imsiVal] = &subsData.AuthenticationSubscription

	err := queries.CreateAmData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}

	logger.NMSLog.Debugln("Config5GUpdateHandle: Insert/Update AuthenticationSubscription ", ueId)
	dbAuthSubsData := &dbModels.AuthenticationSubscription{
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
	err = queries.CreateAuthenticationSubscription(ueId, dbAuthSubsData)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	logger.NMSLog.Infof("Edited Subscriber: %v", ueId)
}

func PatchSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Patch One Subscriber Data")
}

func DeleteSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Delete One Subscriber Data")
	ueId := c.Param("ueId")
	c.JSON(http.StatusNoContent, gin.H{})
	err := queries.DeleteAuthenticationSubscription(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	err = queries.DeleteAmData(ueId)
	if err != nil {
		logger.NMSLog.Warnln(err)
	}
	logger.NMSLog.Infof("Deleted Subscriber: %v", ueId)
}
