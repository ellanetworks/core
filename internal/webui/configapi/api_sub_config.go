// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package configapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/canard/internal/webui/backend/logger"
	"github.com/yeastengine/canard/internal/webui/backend/webui_context"
	"github.com/yeastengine/canard/internal/webui/configmodels"
	"github.com/yeastengine/canard/internal/webui/dbadapter"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	authSubsDataColl = "subscriptionData.authenticationData.authenticationSubscription"
	amDataColl       = "subscriptionData.provisionedData.amData"
	smDataColl       = "subscriptionData.provisionedData.smData"
	smfSelDataColl   = "subscriptionData.provisionedData.smfSelectionSubscriptionData"
	amPolicyDataColl = "policyData.ues.amData"
	smPolicyDataColl = "policyData.ues.smData"
	flowRuleDataColl = "policyData.ues.flowRule"
)

var httpsClient *http.Client

func init() {
	httpsClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func mapToByte(data map[string]interface{}) (ret []byte) {
	ret, _ = json.Marshal(data)
	return
}

func sliceToByte(data []map[string]interface{}) (ret []byte) {
	ret, _ = json.Marshal(data)
	return
}

// seems something which we should move to mongolib
func toBsonM(data interface{}) (ret bson.M) {
	tmp, _ := json.Marshal(data)
	json.Unmarshal(tmp, &ret)
	return
}

func toBsonA(data interface{}) (ret bson.A) {
	tmp, _ := json.Marshal(data)
	json.Unmarshal(tmp, &ret)
	return
}

func setCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
}

func sendResponseToClient(c *gin.Context, response *http.Response) {
	var jsonData interface{}
	json.NewDecoder(response.Body).Decode(&jsonData)
	c.JSON(response.StatusCode, jsonData)
}

func GetSampleJSON(c *gin.Context) {
	setCorsHeader(c)

	logger.WebUILog.Infoln("Get a JSON Example")

	var subsData configmodels.SubsData

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

	subsData = configmodels.SubsData{
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

// Get all subscribers list
func GetSubscribers(c *gin.Context) {
	setCorsHeader(c)

	logger.WebUILog.Infoln("Get All Subscribers List")

	var subsList []configmodels.SubsListIE
	amDataList, errGetMany := dbadapter.CommonDBClient.RestfulAPIGetMany(amDataColl, bson.M{})
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	for _, amData := range amDataList {

		tmp := configmodels.SubsListIE{
			UeId: amData["ueId"].(string),
		}

		if servingPlmnId, plmnIdExists := amData["servingPlmnId"]; plmnIdExists {
			tmp.PlmnID = servingPlmnId.(string)
		}

		subsList = append(subsList, tmp)
	}

	c.JSON(http.StatusOK, subsList)
}

// Get subscriber by IMSI(ueId))
func GetSubscriberByID(c *gin.Context) {
	setCorsHeader(c)

	logger.WebUILog.Infoln("Get One Subscriber Data")

	var subsData configmodels.SubsData

	ueId := c.Param("ueId")

	filterUeIdOnly := bson.M{"ueId": ueId}

	authSubsDataInterface, errGetOneAuth := dbadapter.AuthDBClient.RestfulAPIGetOne(authSubsDataColl, filterUeIdOnly)
	if errGetOneAuth != nil {
		logger.DbLog.Warnln(errGetOneAuth)
	}
	amDataDataInterface, errGetOneAmData := dbadapter.CommonDBClient.RestfulAPIGetOne(amDataColl, filterUeIdOnly)
	if errGetOneAmData != nil {
		logger.DbLog.Warnln(errGetOneAmData)
	}
	smDataDataInterface, errGetManySmData := dbadapter.CommonDBClient.RestfulAPIGetMany(smDataColl, filterUeIdOnly)
	if errGetManySmData != nil {
		logger.DbLog.Warnln(errGetManySmData)
	}
	smfSelDataInterface, errGetOneSmfSel := dbadapter.CommonDBClient.RestfulAPIGetOne(smfSelDataColl, filterUeIdOnly)
	if errGetOneSmfSel != nil {
		logger.DbLog.Warnln(errGetOneSmfSel)
	}
	amPolicyDataInterface, errGetOneAmPol := dbadapter.CommonDBClient.RestfulAPIGetOne(amPolicyDataColl, filterUeIdOnly)
	if errGetOneAmPol != nil {
		logger.DbLog.Warnln(errGetOneAmPol)
	}
	smPolicyDataInterface, errGetManySmPol := dbadapter.CommonDBClient.RestfulAPIGetOne(smPolicyDataColl, filterUeIdOnly)
	if errGetManySmPol != nil {
		logger.DbLog.Warnln(errGetManySmPol)
	}
	var authSubsData models.AuthenticationSubscription
	json.Unmarshal(mapToByte(authSubsDataInterface), &authSubsData)
	var amDataData models.AccessAndMobilitySubscriptionData
	json.Unmarshal(mapToByte(amDataDataInterface), &amDataData)
	var smDataData []models.SessionManagementSubscriptionData
	json.Unmarshal(sliceToByte(smDataDataInterface), &smDataData)
	var smfSelData models.SmfSelectionSubscriptionData
	json.Unmarshal(mapToByte(smfSelDataInterface), &smfSelData)
	var amPolicyData models.AmPolicyData
	json.Unmarshal(mapToByte(amPolicyDataInterface), &amPolicyData)
	var smPolicyData models.SmPolicyData
	json.Unmarshal(mapToByte(smPolicyDataInterface), &smPolicyData)

	subsData = configmodels.SubsData{
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

// Post subscriber by IMSI(ueId)
func PostSubscriberByID(c *gin.Context) {
	setCorsHeader(c)

	var subsOverrideData configmodels.SubsOverrideData
	if err := c.ShouldBindJSON(&subsOverrideData); err != nil {
		logger.WebUILog.Errorln("Post One Subscriber Data - ShouldBindJSON failed ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ueId := c.Param("ueId")

	logger.WebUILog.Infoln("Received Post Subscriber Data from Roc/Simapp: ", ueId)

	authSubsData := models.AuthenticationSubscription{
		AuthenticationManagementField: "8000",
		AuthenticationMethod:          "5G_AKA", // "5G_AKA", "EAP_AKA_PRIME"
		Milenage: &models.Milenage{
			Op: &models.Op{
				EncryptionAlgorithm: 0,
				EncryptionKey:       0,
				OpValue:             "", // Required
			},
		},
		Opc: &models.Opc{
			EncryptionAlgorithm: 0,
			EncryptionKey:       0,
			// OpcValue:            "8e27b6af0e692e750f32667a3b14605d", // Required
		},
		PermanentKey: &models.PermanentKey{
			EncryptionAlgorithm: 0,
			EncryptionKey:       0,
			// PermanentKeyValue:   "8baf473f2f8fd09487cccbd7097c6862", // Required
		},
		// SequenceNumber: "16f3b3f70fc2",
	}

	// override values
	/*if subsOverrideData.PlmnID != "" {
		servingPlmnId = subsOverrideData.PlmnID
	}*/
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

	msg := configmodels.ConfigMessage{
		MsgType:     configmodels.Sub_data,
		MsgMethod:   configmodels.Post_op,
		AuthSubData: &authSubsData,
		Imsi:        ueId,
	}
	configChannel <- &msg
	logger.WebUILog.Infoln("Successfully Added Subscriber Data to ConfigChannel: ", ueId)
}

// Put subscriber by IMSI(ueId) and PlmnID(servingPlmnId)
func PutSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.WebUILog.Infoln("Put One Subscriber Data")

	var subsData configmodels.SubsData
	if err := c.ShouldBindJSON(&subsData); err != nil {
		logger.WebUILog.Panic(err.Error())
	}

	ueId := c.Param("ueId")
	c.JSON(http.StatusNoContent, gin.H{})

	msg := configmodels.ConfigMessage{
		MsgType:     configmodels.Sub_data,
		MsgMethod:   configmodels.Post_op,
		AuthSubData: &subsData.AuthenticationSubscription,
		Imsi:        ueId,
	}
	configChannel <- &msg
	logger.WebUILog.Infoln("Put Subscriber Data complete")
}

// Patch subscriber by IMSI(ueId) and PlmnID(servingPlmnId)
func PatchSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.WebUILog.Infoln("Patch One Subscriber Data")
}

// Delete subscriber by IMSI(ueId)
func DeleteSubscriberByID(c *gin.Context) {
	setCorsHeader(c)
	logger.WebUILog.Infoln("Delete One Subscriber Data")

	ueId := c.Param("ueId")

	c.JSON(http.StatusNoContent, gin.H{})

	msg := configmodels.ConfigMessage{
		MsgType:   configmodels.Sub_data,
		MsgMethod: configmodels.Delete_op,
		Imsi:      ueId,
	}
	configChannel <- &msg
	logger.WebUILog.Infoln("Delete Subscriber Data complete")
}

func GetRegisteredUEContext(c *gin.Context) {
	setCorsHeader(c)

	logger.WebUILog.Infoln("Get Registered UE Context")

	webuiSelf := webui_context.WEBUI_Self()
	webuiSelf.UpdateNfProfiles()

	supi, supiExists := c.Params.Get("supi")

	// TODO: support fetching data from multiple AMFs
	if amfUris := webuiSelf.GetOamUris(models.NfType_AMF); amfUris != nil {
		var requestUri string

		if supiExists {
			requestUri = fmt.Sprintf("%s/namf-oam/v1/registered-ue-context/%s", amfUris[0], supi)
		} else {
			requestUri = fmt.Sprintf("%s/namf-oam/v1/registered-ue-context", amfUris[0])
		}

		resp, err := httpsClient.Get(requestUri)
		if err != nil {
			logger.WebUILog.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}
		sendResponseToClient(c, resp)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"cause": "No AMF Found",
		})
	}
}

func GetUEPDUSessionInfo(c *gin.Context) {
	setCorsHeader(c)

	logger.WebUILog.Infoln("Get UE PDU Session Info")

	webuiSelf := webui_context.WEBUI_Self()
	webuiSelf.UpdateNfProfiles()

	smContextRef, smContextRefExists := c.Params.Get("smContextRef")
	if !smContextRefExists {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	// TODO: support fetching data from multiple SMF
	if smfUris := webuiSelf.GetOamUris(models.NfType_SMF); smfUris != nil {
		requestUri := fmt.Sprintf("%s/nsmf-oam/v1/ue-pdu-session-info/%s", smfUris[0], smContextRef)
		resp, err := httpsClient.Get(requestUri)
		if err != nil {
			logger.WebUILog.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}

		sendResponseToClient(c, resp)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"cause": "No SMF Found",
		})
	}
}
