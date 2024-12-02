package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/nms/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	nmsModels "github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/smf/context"
	"go.mongodb.org/mongo-driver/bson"
)

// Guillaume: This file is way too complex for no good reason.
// Configuration should be handled in a more atomic way.
// This issue is tracked here: https://github.com/yeastengine/ella/issues/205

type Update5GSubscriberMsg struct {
	Msg          *nmsModels.ConfigMessage
	PrevDevGroup *nmsModels.DeviceGroups
	PrevSlice    *nmsModels.Slice
}

var imsiData map[string]*models.AuthenticationSubscription

func init() {
	imsiData = make(map[string]*models.AuthenticationSubscription)
}

func getDeviceGroupByName(name string) *nmsModels.DeviceGroups {
	filter := bson.M{"group-name": name}
	devGroupDataInterface, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.DevGroupDataColl, filter)
	if errGetOne != nil {
		logger.DbLog.Warnln(errGetOne)
	}
	var devGroupData nmsModels.DeviceGroups
	err := json.Unmarshal(mapToByte(devGroupDataInterface), &devGroupData)
	if err != nil {
		logger.DbLog.Errorf("Could not unmarshall device group %v", devGroupDataInterface)
	}
	return &devGroupData
}

func getSlices() []*nmsModels.Slice {
	rawSlices, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.SliceDataColl, nil)
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	var slices []*nmsModels.Slice
	for _, rawSlice := range rawSlices {
		var sliceData nmsModels.Slice
		err := json.Unmarshal(mapToByte(rawSlice), &sliceData)
		if err != nil {
			logger.DbLog.Errorf("Could not unmarshall slice %v", rawSlice)
		}
		slices = append(slices, &sliceData)
	}
	return slices
}

func getSliceByName(name string) *nmsModels.Slice {
	filter := bson.M{"slice-name": name}
	sliceDataInterface, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SliceDataColl, filter)
	if errGetOne != nil {
		logger.DbLog.Warnln(errGetOne)
	}
	var sliceData nmsModels.Slice
	err := json.Unmarshal(mapToByte(sliceDataInterface), &sliceData)
	if err != nil {
		logger.DbLog.Errorf("Could not unmarshall slice %v", sliceDataInterface)
	}
	return &sliceData
}

func getAddedImsisList(group, prevGroup *nmsModels.DeviceGroups) (aimsis []string) {
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

func getDeletedImsisList(group, prevGroup *nmsModels.DeviceGroups) (dimsis []string) {
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
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.AmPolicyDataColl, filter, amPolicyDatBsonA)
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
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.SmPolicyDataColl, filter, smPolicyDatBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateAmProvisionedData(snssai *models.Snssai, qos *nmsModels.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, imsi string) {
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
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.AmDataColl, filter, amDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateSmProvisionedData(snssai *models.Snssai, qos *nmsModels.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, dnn, imsi string) {
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
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.SmDataColl, filter, smDataBsonA)
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
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.SmfSelDataColl, filter, smfSelecDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func isDeviceGroupExistInSlice(msg *Update5GSubscriberMsg) *nmsModels.Slice {
	for name, slice := range getSlices() {
		for _, dgName := range slice.SiteDeviceGroup {
			if dgName == msg.Msg.DevGroupName {
				logger.NMSLog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return slice
			}
		}
	}

	return nil
}

func getDeleteGroupsList(slice, prevSlice *nmsModels.Slice) (names []string) {
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

func updateConfig(confData *Update5GSubscriberMsg) {
	switch confData.Msg.MsgType {
	case nmsModels.Sub_data:
		// check this Imsi is part of any of the devicegroup
		imsi := strings.ReplaceAll(confData.Msg.Imsi, "imsi-", "")
		if confData.Msg.MsgMethod != nmsModels.Delete_op {
			logger.NMSLog.Debugln("Config5GUpdateHandle: Insert/Update AuthenticationSubscription ", imsi)
			filter := bson.M{"ueId": confData.Msg.Imsi}
			authDataBsonA := toBsonM(confData.Msg.AuthSubData)
			authDataBsonA["ueId"] = confData.Msg.Imsi
			_, errPost := db.AuthDBClient.RestfulAPIPost(db.AuthSubsDataColl, filter, authDataBsonA)
			if errPost != nil {
				logger.DbLog.Warnln(errPost)
			}
		} else {
			logger.NMSLog.Debugln("Config5GUpdateHandle: Delete AuthenticationSubscription", imsi)
			filter := bson.M{"ueId": "imsi-" + imsi}
			errDelOne := db.AuthDBClient.RestfulAPIDeleteOne(db.AuthSubsDataColl, filter)
			if errDelOne != nil {
				logger.DbLog.Warnln(errDelOne)
			}
			errDel := db.CommonDBClient.RestfulAPIDeleteOne(db.AmDataColl, filter)
			if errDel != nil {
				logger.DbLog.Warnln(errDel)
			}
		}

	case nmsModels.Device_group:
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
				errDelOneAmPol := db.CommonDBClient.RestfulAPIDeleteOne(db.AmPolicyDataColl, filterImsiOnly)
				if errDelOneAmPol != nil {
					logger.DbLog.Warnln(errDelOneAmPol)
				}
				errDelOneSmPol := db.CommonDBClient.RestfulAPIDeleteOne(db.SmPolicyDataColl, filterImsiOnly)
				if errDelOneSmPol != nil {
					logger.DbLog.Warnln(errDelOneSmPol)
				}
				errDelOneAmData := db.CommonDBClient.RestfulAPIDeleteOne(db.AmDataColl, filter)
				if errDelOneAmData != nil {
					logger.DbLog.Warnln(errDelOneAmData)
				}
				errDelOneSmData := db.CommonDBClient.RestfulAPIDeleteOne(db.SmDataColl, filter)
				if errDelOneSmData != nil {
					logger.DbLog.Warnln(errDelOneSmData)
				}
				errDelOneSmfSel := db.CommonDBClient.RestfulAPIDeleteOne(db.SmfSelDataColl, filter)
				if errDelOneSmfSel != nil {
					logger.DbLog.Warnln(errDelOneSmfSel)
				}
			}
		}

	case nmsModels.Network_slice:
		logger.NMSLog.Debugln("Insert/Update Network Slice")
		slice := confData.Msg.Slice
		if slice == nil && confData.PrevSlice != nil {
			logger.NMSLog.Debugln("Deleted Slice: ", confData.PrevSlice)
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
					errDelOneAmPol := db.CommonDBClient.RestfulAPIDeleteOne(db.AmPolicyDataColl, filterImsiOnly)
					if errDelOneAmPol != nil {
						logger.DbLog.Warnln(errDelOneAmPol)
					}
					errDelOneSmPol := db.CommonDBClient.RestfulAPIDeleteOne(db.SmPolicyDataColl, filterImsiOnly)
					if errDelOneSmPol != nil {
						logger.DbLog.Warnln(errDelOneSmPol)
					}
					errDelOneAmData := db.CommonDBClient.RestfulAPIDeleteOne(db.AmDataColl, filter)
					if errDelOneAmData != nil {
						logger.DbLog.Warnln(errDelOneAmData)
					}
					errDelOneSmData := db.CommonDBClient.RestfulAPIDeleteOne(db.SmDataColl, filter)
					if errDelOneSmData != nil {
						logger.DbLog.Warnln(errDelOneSmData)
					}
					errDelOneSmfSel := db.CommonDBClient.RestfulAPIDeleteOne(db.SmfSelDataColl, filter)
					if errDelOneSmfSel != nil {
						logger.DbLog.Warnln(errDelOneSmfSel)
					}
				}
			}
		}
	}
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

// // seems something which we should move to mongolib
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

func SnssaiModelsToHex(snssai models.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}

func updateSMF() {
	networkSlices := make([]nmsModels.Slice, 0)
	networkSliceNames := ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := GetNetworkSliceByName2(networkSliceName)
		networkSlices = append(networkSlices, networkSlice)
	}
	deviceGroups := make([]nmsModels.DeviceGroups, 0)
	deviceGroupNames := ListDeviceGroups()
	for _, deviceGroupName := range deviceGroupNames {
		deviceGroup := GetDeviceGroupByName2(deviceGroupName)
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
