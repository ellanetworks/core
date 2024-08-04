package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/omec-project/openapi/models"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/webui/backend/logger"
	"github.com/yeastengine/ella/internal/webui/configmodels"
	"github.com/yeastengine/ella/internal/webui/dbadapter"
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
	devGroupDataColl = "webconsoleData.snapshots.devGroupData"
	sliceDataColl    = "webconsoleData.snapshots.sliceData"
	gnbDataColl      = "webconsoleData.snapshots.gnbData"
)

var configLog *logrus.Entry

func init() {
	configLog = logger.ConfigLog
}

type Update5GSubscriberMsg struct {
	Msg          *configmodels.ConfigMessage
	PrevDevGroup *configmodels.DeviceGroups
	PrevSlice    *configmodels.Slice
}

var rwLock sync.RWMutex

var imsiData map[string]*models.AuthenticationSubscription

func init() {
	imsiData = make(map[string]*models.AuthenticationSubscription)
}

func configHandler(configMsgChan chan *configmodels.ConfigMessage, configReceived chan bool) {
	// Start Goroutine which will listens for subscriber config updates
	// and update the mongoDB. Only for 5G
	subsUpdateChan := make(chan *Update5GSubscriberMsg, 10)
	go Config5GUpdateHandle(subsUpdateChan)
	firstConfigRcvd := firstConfigReceived()
	if firstConfigRcvd {
		configReceived <- true
	}
	for {
		configLog.Infoln("Waiting for configuration event ")
		configMsg := <-configMsgChan
		// configLog.Infof("Received configuration event %v ", configMsg)
		if configMsg.MsgType == configmodels.Sub_data {
			imsiVal := strings.ReplaceAll(configMsg.Imsi, "imsi-", "")
			configLog.Infoln("Received imsi from config channel: ", imsiVal)
			rwLock.Lock()
			imsiData[imsiVal] = configMsg.AuthSubData
			rwLock.Unlock()
			configLog.Infof("Received Imsi [%v] configuration from config channel", configMsg.Imsi)
			handleSubscriberPost(configMsg)
			var configUMsg Update5GSubscriberMsg
			configUMsg.Msg = configMsg
			subsUpdateChan <- &configUMsg
		}

		if configMsg.MsgMethod == configmodels.Post_op || configMsg.MsgMethod == configmodels.Put_op {
			if !firstConfigRcvd && (configMsg.MsgType == configmodels.Device_group || configMsg.MsgType == configmodels.Network_slice) {
				configLog.Debugln("First config received from ROC")
				firstConfigRcvd = true
				configReceived <- true
			}

			// configLog.Infoln("Received msg from configApi package ", configMsg)
			// update config snapshot
			if configMsg.DevGroup != nil {
				configLog.Infof("Received Device Group [%v] configuration from config channel", configMsg.DevGroupName)
				handleDeviceGroupPost(configMsg, subsUpdateChan)
			}

			if configMsg.Slice != nil {
				configLog.Infof("Received Slice [%v] configuration from config channel", configMsg.SliceName)
				handleNetworkSlicePost(configMsg, subsUpdateChan)
			}

			if configMsg.Gnb != nil {
				configLog.Infof("Received gNB [%v] configuration from config channel", configMsg.GnbName)
				handleGnbPost(configMsg)
			}

			// loop through all clients and send this message to all clients
			if len(clientNFPool) == 0 {
				configLog.Infoln("No client available. No need to send config")
			}
			for _, client := range clientNFPool {
				configLog.Infoln("Push config for client : ", client.id)
				client.outStandingPushConfig <- configMsg
			}
		} else {
			var config5gMsg Update5GSubscriberMsg
			if configMsg.MsgType == configmodels.Inventory {
				if configMsg.GnbName != "" {
					configLog.Infof("Received delete gNB [%v] from config channel", configMsg.GnbName)
					handleGnbDelete(configMsg)
				}
			} else if configMsg.MsgType != configmodels.Sub_data {
				rwLock.Lock()
				// update config snapshot
				if configMsg.DevGroup == nil {
					configLog.Infof("Received delete Device Group [%v] from config channel", configMsg.DevGroupName)
					config5gMsg.PrevDevGroup = getDeviceGroupByName(configMsg.DevGroupName)
					filter := bson.M{"group-name": configMsg.DevGroupName}
					errDelOne := dbadapter.CommonDBClient.RestfulAPIDeleteOne(devGroupDataColl, filter)
					if errDelOne != nil {
						logger.DbLog.Warnln(errDelOne)
					}
				}

				if configMsg.Slice == nil {
					configLog.Infof("Received delete Slice [%v] from config channel", configMsg.SliceName)
					config5gMsg.PrevSlice = getSliceByName(configMsg.SliceName)
					filter := bson.M{"slice-name": configMsg.SliceName}
					errDelOne := dbadapter.CommonDBClient.RestfulAPIDeleteOne(sliceDataColl, filter)
					if errDelOne != nil {
						logger.DbLog.Warnln(errDelOne)
					}
				}
				rwLock.Unlock()
			} else {
				configLog.Infof("Received delete Subscriber [%v] from config channel", configMsg.Imsi)
			}
			config5gMsg.Msg = configMsg
			subsUpdateChan <- &config5gMsg
			// loop through all clients and send this message to all clients
			if len(clientNFPool) == 0 {
				configLog.Infoln("No client available. No need to send config")
			}
			for _, client := range clientNFPool {
				configLog.Infoln("Push config for client : ", client.id)
				client.outStandingPushConfig <- configMsg
			}
		}
	}
}

func handleSubscriberPost(configMsg *configmodels.ConfigMessage) {
	rwLock.Lock()
	basicAmData := map[string]interface{}{
		"ueId": configMsg.Imsi,
	}
	filter := bson.M{"ueId": configMsg.Imsi}
	basicDataBson := toBsonM(basicAmData)
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(amDataColl, filter, basicDataBson)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	rwLock.Unlock()
}

func handleDeviceGroupPost(configMsg *configmodels.ConfigMessage, subsUpdateChan chan *Update5GSubscriberMsg) {
	rwLock.Lock()
	var config5gMsg Update5GSubscriberMsg
	config5gMsg.Msg = configMsg
	config5gMsg.PrevDevGroup = getDeviceGroupByName(configMsg.DevGroupName)
	subsUpdateChan <- &config5gMsg
	filter := bson.M{"group-name": configMsg.DevGroupName}
	devGroupDataBsonA := toBsonM(configMsg.DevGroup)
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(devGroupDataColl, filter, devGroupDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	rwLock.Unlock()
}

func handleNetworkSlicePost(configMsg *configmodels.ConfigMessage, subsUpdateChan chan *Update5GSubscriberMsg) {
	rwLock.Lock()
	var config5gMsg Update5GSubscriberMsg
	config5gMsg.Msg = configMsg
	config5gMsg.PrevSlice = getSliceByName(configMsg.SliceName)
	subsUpdateChan <- &config5gMsg
	filter := bson.M{"slice-name": configMsg.SliceName}
	sliceDataBsonA := toBsonM(configMsg.Slice)
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(sliceDataColl, filter, sliceDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	rwLock.Unlock()
}

func handleGnbPost(configMsg *configmodels.ConfigMessage) {
	rwLock.Lock()
	filter := bson.M{"name": configMsg.GnbName}
	gnbDataBson := toBsonM(configMsg.Gnb)
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(gnbDataColl, filter, gnbDataBson)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	rwLock.Unlock()
}

func handleGnbDelete(configMsg *configmodels.ConfigMessage) {
	rwLock.Lock()
	filter := bson.M{"name": configMsg.GnbName}
	errDelOne := dbadapter.CommonDBClient.RestfulAPIDeleteOne(gnbDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	rwLock.Unlock()
}

func firstConfigReceived() bool {
	return len(getDeviceGroups()) > 0 || len(getSlices()) > 0
}

func getDeviceGroups() []*configmodels.DeviceGroups {
	rawDeviceGroups, errGetMany := dbadapter.CommonDBClient.RestfulAPIGetMany(devGroupDataColl, nil)
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	var deviceGroups []*configmodels.DeviceGroups
	for _, rawDevGroup := range rawDeviceGroups {
		var devGroupData configmodels.DeviceGroups
		err := json.Unmarshal(mapToByte(rawDevGroup), &devGroupData)
		if err != nil {
			logger.DbLog.Errorf("Could not unmarshall device group %v", rawDevGroup)
		}
		deviceGroups = append(deviceGroups, &devGroupData)
	}
	return deviceGroups
}

func getDeviceGroupByName(name string) *configmodels.DeviceGroups {
	filter := bson.M{"group-name": name}
	devGroupDataInterface, errGetOne := dbadapter.CommonDBClient.RestfulAPIGetOne(devGroupDataColl, filter)
	if errGetOne != nil {
		logger.DbLog.Warnln(errGetOne)
	}
	var devGroupData configmodels.DeviceGroups
	err := json.Unmarshal(mapToByte(devGroupDataInterface), &devGroupData)
	if err != nil {
		logger.DbLog.Errorf("Could not unmarshall device group %v", devGroupDataInterface)
	}
	return &devGroupData
}

func getSlices() []*configmodels.Slice {
	rawSlices, errGetMany := dbadapter.CommonDBClient.RestfulAPIGetMany(sliceDataColl, nil)
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	var slices []*configmodels.Slice
	for _, rawSlice := range rawSlices {
		var sliceData configmodels.Slice
		err := json.Unmarshal(mapToByte(rawSlice), &sliceData)
		if err != nil {
			logger.DbLog.Errorf("Could not unmarshall slice %v", rawSlice)
		}
		slices = append(slices, &sliceData)
	}
	return slices
}

func getSliceByName(name string) *configmodels.Slice {
	filter := bson.M{"slice-name": name}
	sliceDataInterface, errGetOne := dbadapter.CommonDBClient.RestfulAPIGetOne(sliceDataColl, filter)
	if errGetOne != nil {
		logger.DbLog.Warnln(errGetOne)
	}
	var sliceData configmodels.Slice
	err := json.Unmarshal(mapToByte(sliceDataInterface), &sliceData)
	if err != nil {
		logger.DbLog.Errorf("Could not unmarshall slice %v", sliceDataInterface)
	}
	return &sliceData
}

func getAddedImsisList(group, prevGroup *configmodels.DeviceGroups) (aimsis []string) {
	if group == nil {
		return
	}
	for _, imsi := range group.Imsis {
		if prevGroup == nil {
			if imsiData[imsi] != nil {
				aimsis = append(aimsis, imsi)
			}
		} else {
			var found bool
			for _, pimsi := range prevGroup.Imsis {
				if pimsi == imsi {
					found = true
				}
			}

			if !found {
				aimsis = append(aimsis, imsi)
			}
		}
	}

	return
}

func getDeletedImsisList(group, prevGroup *configmodels.DeviceGroups) (dimsis []string) {
	if prevGroup == nil {
		return
	}

	if group == nil {
		return prevGroup.Imsis
	}

	for _, pimsi := range prevGroup.Imsis {
		var found bool
		for _, imsi := range group.Imsis {
			if pimsi == imsi {
				found = true
			}
		}

		if !found {
			dimsis = append(dimsis, pimsi)
		}
	}

	return
}

func updateAmPolicyData(imsi string) {
	// ampolicydata
	var amPolicy models.AmPolicyData
	amPolicy.SubscCats = append(amPolicy.SubscCats, "free5gc")
	amPolicyDatBsonA := toBsonM(amPolicy)
	amPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(amPolicyDataColl, filter, amPolicyDatBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateSmPolicyData(snssai *models.Snssai, dnn string, imsi string) {
	var smPolicyData models.SmPolicyData
	var smPolicySnssaiData models.SmPolicySnssaiData
	dnnData := map[string]models.SmPolicyDnnData{
		dnn: {
			Dnn: dnn,
		},
	}
	// smpolicydata
	smPolicySnssaiData.Snssai = snssai
	smPolicySnssaiData.SmPolicyDnnData = dnnData
	smPolicyData.SmPolicySnssaiData = make(map[string]models.SmPolicySnssaiData)
	smPolicyData.SmPolicySnssaiData[SnssaiModelsToHex(*snssai)] = smPolicySnssaiData
	smPolicyDatBsonA := toBsonM(smPolicyData)
	smPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(smPolicyDataColl, filter, smPolicyDatBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateAmProvisionedData(snssai *models.Snssai, qos *configmodels.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, imsi string) {
	amData := models.AccessAndMobilitySubscriptionData{
		Gpsis: []string{
			"msisdn-0900000000",
		},
		Nssai: &models.Nssai{
			DefaultSingleNssais: []models.Snssai{*snssai},
			SingleNssais:        []models.Snssai{*snssai},
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: convertToString(uint64(qos.DnnMbrDownlink)),
			Uplink:   convertToString(uint64(qos.DnnMbrUplink)),
		},
	}
	amDataBsonA := toBsonM(amData)
	amDataBsonA["ueId"] = "imsi-" + imsi
	amDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{
		"ueId": "imsi-" + imsi,
		"$or": []bson.M{
			{"servingPlmnId": mcc + mnc},
			{"servingPlmnId": bson.M{"$exists": false}},
		},
	}
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(amDataColl, filter, amDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateSmProvisionedData(snssai *models.Snssai, qos *configmodels.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, dnn, imsi string) {
	// TODO smData
	smData := models.SessionManagementSubscriptionData{
		SingleNssai: snssai,
		DnnConfigurations: map[string]models.DnnConfiguration{
			dnn: {
				PduSessionTypes: &models.PduSessionTypes{
					DefaultSessionType:  models.PduSessionType_IPV4,
					AllowedSessionTypes: []models.PduSessionType{models.PduSessionType_IPV4},
				},
				SscModes: &models.SscModes{
					DefaultSscMode: models.SscMode__1,
					AllowedSscModes: []models.SscMode{
						"SSC_MODE_2",
						"SSC_MODE_3",
					},
				},
				SessionAmbr: &models.Ambr{
					Downlink: convertToString(uint64(qos.DnnMbrDownlink)),
					Uplink:   convertToString(uint64(qos.DnnMbrUplink)),
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
	}
	smDataBsonA := toBsonM(smData)
	smDataBsonA["ueId"] = "imsi-" + imsi
	smDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(smDataColl, filter, smDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateSmfSelectionProviosionedData(snssai *models.Snssai, mcc, mnc, dnn, imsi string) {
	smfSelData := models.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: map[string]models.SnssaiInfo{
			SnssaiModelsToHex(*snssai): {
				DnnInfos: []models.DnnInfo{
					{
						Dnn: dnn,
					},
				},
			},
		},
	}
	smfSelecDataBsonA := toBsonM(smfSelData)
	smfSelecDataBsonA["ueId"] = "imsi-" + imsi
	smfSelecDataBsonA["servingPlmnId"] = mcc + mnc
	filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
	_, errPost := dbadapter.CommonDBClient.RestfulAPIPost(smfSelDataColl, filter, smfSelecDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func isDeviceGroupExistInSlice(msg *Update5GSubscriberMsg) *configmodels.Slice {
	for name, slice := range getSlices() {
		for _, dgName := range slice.SiteDeviceGroup {
			if dgName == msg.Msg.DevGroupName {
				logger.WebUILog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return slice
			}
		}
	}

	return nil
}

func getAddedGroupsList(slice, prevSlice *configmodels.Slice) (names []string) {
	return getDeleteGroupsList(prevSlice, slice)
}

func getDeleteGroupsList(slice, prevSlice *configmodels.Slice) (names []string) {
	for prevSlice == nil {
		return
	}

	if slice != nil {
		for _, pdgName := range prevSlice.SiteDeviceGroup {
			var found bool
			for _, dgName := range slice.SiteDeviceGroup {
				if dgName == pdgName {
					found = true
					break
				}
			}
			if !found {
				names = append(names, pdgName)
			}
		}
	} else {
		names = append(names, prevSlice.SiteDeviceGroup...)
	}

	return
}

func Config5GUpdateHandle(confChan chan *Update5GSubscriberMsg) {
	for confData := range confChan {
		switch confData.Msg.MsgType {
		case configmodels.Sub_data:
			rwLock.RLock()
			// check this Imsi is part of any of the devicegroup
			imsi := strings.ReplaceAll(confData.Msg.Imsi, "imsi-", "")
			if confData.Msg.MsgMethod != configmodels.Delete_op {
				logger.WebUILog.Debugln("Insert/Update AuthenticationSubscription ", imsi)
				filter := bson.M{"ueId": confData.Msg.Imsi}
				authDataBsonA := toBsonM(confData.Msg.AuthSubData)
				authDataBsonA["ueId"] = confData.Msg.Imsi
				_, errPost := dbadapter.AuthDBClient.RestfulAPIPost(authSubsDataColl, filter, authDataBsonA)
				if errPost != nil {
					logger.DbLog.Warnln(errPost)
				}
			} else {
				logger.WebUILog.Debugln("Delete AuthenticationSubscription", imsi)
				filter := bson.M{"ueId": "imsi-" + imsi}
				errDelOne := dbadapter.AuthDBClient.RestfulAPIDeleteOne(authSubsDataColl, filter)
				if errDelOne != nil {
					logger.DbLog.Warnln(errDelOne)
				}
				errDel := dbadapter.CommonDBClient.RestfulAPIDeleteOne(amDataColl, filter)
				if errDel != nil {
					logger.DbLog.Warnln(errDel)
				}
			}
			rwLock.RUnlock()

		case configmodels.Device_group:
			rwLock.RLock()
			/* is this devicegroup part of any existing slice */
			slice := isDeviceGroupExistInSlice(confData)
			if slice != nil {
				sVal, err := strconv.ParseUint(slice.SliceId.Sst, 10, 32)
				if err != nil {
					logger.DbLog.Errorf("Could not parse SST %v", slice.SliceId.Sst)
				}
				snssai := &models.Snssai{
					Sd:  slice.SliceId.Sd,
					Sst: int32(sVal),
				}

				aimsis := getAddedImsisList(confData.Msg.DevGroup, confData.PrevDevGroup)
				for _, imsi := range aimsis {
					dnn := confData.Msg.DevGroup.IpDomainExpanded.Dnn
					updateAmPolicyData(imsi)
					updateSmPolicyData(snssai, dnn, imsi)
					updateAmProvisionedData(snssai, confData.Msg.DevGroup.IpDomainExpanded.UeDnnQos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, imsi)
					updateSmProvisionedData(snssai, confData.Msg.DevGroup.IpDomainExpanded.UeDnnQos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
					updateSmfSelectionProviosionedData(snssai, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
				}

				dimsis := getDeletedImsisList(confData.Msg.DevGroup, confData.PrevDevGroup)
				for _, imsi := range dimsis {
					mcc := slice.SiteInfo.Plmn.Mcc
					mnc := slice.SiteInfo.Plmn.Mnc
					filterImsiOnly := bson.M{"ueId": "imsi-" + imsi}
					filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
					errDelOneAmPol := dbadapter.CommonDBClient.RestfulAPIDeleteOne(amPolicyDataColl, filterImsiOnly)
					if errDelOneAmPol != nil {
						logger.DbLog.Warnln(errDelOneAmPol)
					}
					errDelOneSmPol := dbadapter.CommonDBClient.RestfulAPIDeleteOne(smPolicyDataColl, filterImsiOnly)
					if errDelOneSmPol != nil {
						logger.DbLog.Warnln(errDelOneSmPol)
					}
					errDelOneAmData := dbadapter.CommonDBClient.RestfulAPIDeleteOne(amDataColl, filter)
					if errDelOneAmData != nil {
						logger.DbLog.Warnln(errDelOneAmData)
					}
					errDelOneSmData := dbadapter.CommonDBClient.RestfulAPIDeleteOne(smDataColl, filter)
					if errDelOneSmData != nil {
						logger.DbLog.Warnln(errDelOneSmData)
					}
					errDelOneSmfSel := dbadapter.CommonDBClient.RestfulAPIDeleteOne(smfSelDataColl, filter)
					if errDelOneSmfSel != nil {
						logger.DbLog.Warnln(errDelOneSmfSel)
					}
				}
			}
			rwLock.RUnlock()

		case configmodels.Network_slice:
			rwLock.RLock()
			logger.WebUILog.Debugln("Insert/Update Network Slice")
			slice := confData.Msg.Slice
			if slice == nil && confData.PrevSlice != nil {
				logger.WebUILog.Debugln("Deleted Slice: ", confData.PrevSlice)
			}
			if slice != nil {
				sVal, err := strconv.ParseUint(slice.SliceId.Sst, 10, 32)
				if err != nil {
					logger.DbLog.Errorf("Could not parse SST %v", slice.SliceId.Sst)
				}
				snssai := &models.Snssai{
					Sd:  slice.SliceId.Sd,
					Sst: int32(sVal),
				}
				for _, dgName := range slice.SiteDeviceGroup {
					configLog.Infoln("dgName : ", dgName)
					devGroupConfig := getDeviceGroupByName(dgName)
					if devGroupConfig != nil {
						for _, imsi := range devGroupConfig.Imsis {
							dnn := devGroupConfig.IpDomainExpanded.Dnn
							mcc := slice.SiteInfo.Plmn.Mcc
							mnc := slice.SiteInfo.Plmn.Mnc
							updateAmPolicyData(imsi)
							updateSmPolicyData(snssai, dnn, imsi)
							updateAmProvisionedData(snssai, devGroupConfig.IpDomainExpanded.UeDnnQos, mcc, mnc, imsi)
							updateSmProvisionedData(snssai, devGroupConfig.IpDomainExpanded.UeDnnQos, mcc, mnc, dnn, imsi)
							updateSmfSelectionProviosionedData(snssai, mcc, mnc, dnn, imsi)
						}
					}
				}
			}

			dgnames := getDeleteGroupsList(slice, confData.PrevSlice)
			for _, dgname := range dgnames {
				devGroupConfig := getDeviceGroupByName(dgname)
				if devGroupConfig != nil {
					for _, imsi := range devGroupConfig.Imsis {
						mcc := confData.PrevSlice.SiteInfo.Plmn.Mcc
						mnc := confData.PrevSlice.SiteInfo.Plmn.Mnc
						filterImsiOnly := bson.M{"ueId": "imsi-" + imsi}
						filter := bson.M{"ueId": "imsi-" + imsi, "servingPlmnId": mcc + mnc}
						errDelOneAmPol := dbadapter.CommonDBClient.RestfulAPIDeleteOne(amPolicyDataColl, filterImsiOnly)
						if errDelOneAmPol != nil {
							logger.DbLog.Warnln(errDelOneAmPol)
						}
						errDelOneSmPol := dbadapter.CommonDBClient.RestfulAPIDeleteOne(smPolicyDataColl, filterImsiOnly)
						if errDelOneSmPol != nil {
							logger.DbLog.Warnln(errDelOneSmPol)
						}
						errDelOneAmData := dbadapter.CommonDBClient.RestfulAPIDeleteOne(amDataColl, filter)
						if errDelOneAmData != nil {
							logger.DbLog.Warnln(errDelOneAmData)
						}
						errDelOneSmData := dbadapter.CommonDBClient.RestfulAPIDeleteOne(smDataColl, filter)
						if errDelOneSmData != nil {
							logger.DbLog.Warnln(errDelOneSmData)
						}
						errDelOneSmfSel := dbadapter.CommonDBClient.RestfulAPIDeleteOne(smfSelDataColl, filter)
						if errDelOneSmfSel != nil {
							logger.DbLog.Warnln(errDelOneSmfSel)
						}
					}
				}
			}
			rwLock.RUnlock()
		}
	} // end of for loop
}

func convertToString(val uint64) string {
	var mbVal, gbVal, kbVal uint64
	kbVal = val / 1000
	mbVal = val / 1000000
	gbVal = val / 1000000000
	var retStr string
	if gbVal != 0 {
		retStr = strconv.FormatUint(gbVal, 10) + " Gbps"
	} else if mbVal != 0 {
		retStr = strconv.FormatUint(mbVal, 10) + " Mbps"
	} else if kbVal != 0 {
		retStr = strconv.FormatUint(kbVal, 10) + " Kbps"
	} else {
		retStr = strconv.FormatUint(val, 10) + " bps"
	}

	return retStr
}

// seems something which we should move to mongolib
func toBsonM(data interface{}) (ret bson.M) {
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.DbLog.Errorln("Could not marshall data")
		return nil
	}
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.DbLog.Errorln("Could not unmarshall data")
		return nil
	}
	return ret
}

func mapToByte(data map[string]interface{}) (ret []byte) {
	ret, err := json.Marshal(data)
	if err != nil {
		logger.DbLog.Errorln("Could not marshall data")
		return nil
	}
	return ret
}

func SnssaiModelsToHex(snssai models.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}
