package server

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	openAPIModels "github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/nms/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/smf/context"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	KPS = 1000
	MPS = 1000000
	GPS = 1000000000
)

var configLog *logrus.Entry

var imsiData map[string]*openAPIModels.AuthenticationSubscription

func init() {
	configLog = logger.ConfigLog
	imsiData = make(map[string]*openAPIModels.AuthenticationSubscription)
}

func DeviceGroupDeleteHandler(c *gin.Context) bool {
	var groupName string
	var exists bool
	if groupName, exists = c.Params.Get("group-name"); exists {
		configLog.Infof("Received Delete Group %v from Roc/simapp", groupName)
	}
	prevDevGroup := getDeviceGroupByName(groupName)
	filter := bson.M{"group-name": groupName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.DevGroupDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	updateDeviceGroupConfig(groupName, nil, prevDevGroup)
	updateSMF()
	configLog.Infof("Deleted Device Group: %v", groupName)
	return true
}

func convertToBps(val int64, unit string) (bitrate int64) {
	if strings.EqualFold(unit, "bps") {
		bitrate = val
	} else if strings.EqualFold(unit, "kbps") {
		bitrate = val * KPS
	} else if strings.EqualFold(unit, "mbps") {
		bitrate = val * MPS
	} else if strings.EqualFold(unit, "gbps") {
		bitrate = val * GPS
	}
	// default consider it as bps
	return bitrate
}

func DeviceGroupPostHandler(c *gin.Context, msgOp int) bool {
	var groupName string
	var exists bool
	if groupName, exists = c.Params.Get("group-name"); exists {
		configLog.Infof("Received group %v", groupName)
	}

	var err error
	var request models.DeviceGroups
	s := strings.Split(c.GetHeader("Content-Type"), ";")
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(&request)
	}
	if err != nil {
		configLog.Infof(" err %v", err)
		return false
	}
	req := httpwrapper.NewRequest(c.Request, request)

	procReq := req.Body.(models.DeviceGroups)
	ipdomain := &procReq.IpDomainExpanded

	if ipdomain.UeDnnQos != nil {
		ipdomain.UeDnnQos.DnnMbrDownlink = convertToBps(ipdomain.UeDnnQos.DnnMbrDownlink, ipdomain.UeDnnQos.BitrateUnit)
		if ipdomain.UeDnnQos.DnnMbrDownlink < 0 {
			ipdomain.UeDnnQos.DnnMbrDownlink = math.MaxInt64
		}
		ipdomain.UeDnnQos.DnnMbrUplink = convertToBps(ipdomain.UeDnnQos.DnnMbrUplink, ipdomain.UeDnnQos.BitrateUnit)
		if ipdomain.UeDnnQos.DnnMbrUplink < 0 {
			ipdomain.UeDnnQos.DnnMbrUplink = math.MaxInt64
		}
	}

	procReq.DeviceGroupName = groupName
	prevDevGroup := getDeviceGroupByName(groupName)
	updateDeviceGroupConfig(groupName, &procReq, prevDevGroup)
	filter := bson.M{"group-name": groupName}
	devGroupDataBsonA := toBsonM(&procReq)
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.DevGroupDataColl, filter, devGroupDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	updateSMF()
	configLog.Infof("Created Device Group: %v", groupName)
	return true
}

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	var sliceName string
	var exists bool
	if sliceName, exists = c.Params.Get("slice-name"); exists {
		configLog.Infof("Received Deleted slice : %v from Roc/simapp", sliceName)
	}
	prevSlice := getSliceByName(sliceName)
	filter := bson.M{"slice-name": sliceName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.SliceDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	dgnames := getDeleteGroupsList(nil, prevSlice)
	for _, dgname := range dgnames {
		devGroupConfig := getDeviceGroupByName(dgname)
		if devGroupConfig != nil {
			for _, imsi := range devGroupConfig.Imsis {
				mcc := prevSlice.SiteInfo.Plmn.Mcc
				mnc := prevSlice.SiteInfo.Plmn.Mnc
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
	updateSMF()
	configLog.Infof("Deleted Network Slice: %v", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	var sliceName string
	var exists bool
	if sliceName, exists = c.Params.Get("slice-name"); exists {
		configLog.Infof("Received slice : %v", sliceName)
	}

	var err error
	var request models.Slice
	s := strings.Split(c.GetHeader("Content-Type"), ";")
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(&request)
	}
	if err != nil {
		return false
	}

	req := httpwrapper.NewRequest(c.Request, request)

	procReq := req.Body.(models.Slice)
	group := procReq.SiteDeviceGroup
	slices.Sort(group)

	for index := range procReq.ApplicationFilteringRules {
		ul := procReq.ApplicationFilteringRules[index].AppMbrUplink
		dl := procReq.ApplicationFilteringRules[index].AppMbrDownlink
		unit := procReq.ApplicationFilteringRules[index].BitrateUnit

		bitrate := convertToBps(int64(ul), unit)
		if bitrate < 0 || bitrate > math.MaxInt32 {
			procReq.ApplicationFilteringRules[index].AppMbrUplink = math.MaxInt32
		} else {
			procReq.ApplicationFilteringRules[index].AppMbrUplink = int32(bitrate)
		}

		bitrate = convertToBps(int64(dl), unit)
		if bitrate < 0 || bitrate > math.MaxInt32 {
			procReq.ApplicationFilteringRules[index].AppMbrDownlink = math.MaxInt32
		} else {
			procReq.ApplicationFilteringRules[index].AppMbrDownlink = int32(bitrate)
		}
	}

	procReq.SliceName = sliceName
	sVal, err := strconv.ParseUint(procReq.SliceId.Sst, 10, 32)
	if err != nil {
		logger.DbLog.Errorf("Could not parse SST %v", procReq.SliceId.Sst)
	}
	snssai := &openAPIModels.Snssai{
		Sd:  procReq.SliceId.Sd,
		Sst: int32(sVal),
	}
	for _, dgName := range procReq.SiteDeviceGroup {
		configLog.Infoln("dgName : ", dgName)
		devGroupConfig := getDeviceGroupByName(dgName)
		if devGroupConfig != nil {
			for _, imsi := range devGroupConfig.Imsis {
				dnn := devGroupConfig.IpDomainExpanded.Dnn
				mcc := procReq.SiteInfo.Plmn.Mcc
				mnc := procReq.SiteInfo.Plmn.Mnc
				updateAmPolicyData(imsi)
				updateSmPolicyData(snssai, dnn, imsi)
				updateAmProvisionedData(snssai, devGroupConfig.IpDomainExpanded.UeDnnQos, mcc, mnc, imsi)
				updateSmProvisionedData(snssai, devGroupConfig.IpDomainExpanded.UeDnnQos, mcc, mnc, dnn, imsi)
				updateSmfSelectionProviosionedData(snssai, mcc, mnc, dnn, imsi)
			}
		}
	}
	filter := bson.M{"slice-name": sliceName}
	sliceDataBsonA := toBsonM(&procReq)
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.SliceDataColl, filter, sliceDataBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
	updateSMF()
	configLog.Infof("Created Network Slice: %v", sliceName)
	return true
}

func getDeviceGroupByName(name string) *models.DeviceGroups {
	filter := bson.M{"group-name": name}
	devGroupDataInterface, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.DevGroupDataColl, filter)
	if errGetOne != nil {
		logger.DbLog.Warnln(errGetOne)
	}
	var devGroupData models.DeviceGroups
	err := json.Unmarshal(mapToByte(devGroupDataInterface), &devGroupData)
	if err != nil {
		logger.DbLog.Errorf("Could not unmarshall device group %v", devGroupDataInterface)
	}
	return &devGroupData
}

func getSlices() []*models.Slice {
	rawSlices, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.SliceDataColl, nil)
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	var slices []*models.Slice
	for _, rawSlice := range rawSlices {
		var sliceData models.Slice
		err := json.Unmarshal(mapToByte(rawSlice), &sliceData)
		if err != nil {
			logger.DbLog.Errorf("Could not unmarshall slice %v", rawSlice)
		}
		slices = append(slices, &sliceData)
	}
	return slices
}

func getSliceByName(name string) *models.Slice {
	filter := bson.M{"slice-name": name}
	sliceDataInterface, errGetOne := db.CommonDBClient.RestfulAPIGetOne(db.SliceDataColl, filter)
	if errGetOne != nil {
		logger.DbLog.Warnln(errGetOne)
	}
	var sliceData models.Slice
	err := json.Unmarshal(mapToByte(sliceDataInterface), &sliceData)
	if err != nil {
		logger.DbLog.Errorf("Could not unmarshall slice %v", sliceDataInterface)
	}
	return &sliceData
}

func getAddedImsisList(group, prevGroup *models.DeviceGroups) (aimsis []string) {
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

func getDeletedImsisList(group, prevGroup *models.DeviceGroups) (dimsis []string) {
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
	var amPolicy openAPIModels.AmPolicyData
	amPolicy.SubscCats = append(amPolicy.SubscCats, "free5gc")
	amPolicyDatBsonA := toBsonM(amPolicy)
	amPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.AmPolicyDataColl, filter, amPolicyDatBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateSmPolicyData(snssai *openAPIModels.Snssai, dnn string, imsi string) {
	var smPolicyData openAPIModels.SmPolicyData
	var smPolicySnssaiData openAPIModels.SmPolicySnssaiData
	dnnData := map[string]openAPIModels.SmPolicyDnnData{
		dnn: {
			Dnn: dnn,
		},
	}
	// smpolicydata
	smPolicySnssaiData.Snssai = snssai
	smPolicySnssaiData.SmPolicyDnnData = dnnData
	smPolicyData.SmPolicySnssaiData = make(map[string]openAPIModels.SmPolicySnssaiData)
	smPolicyData.SmPolicySnssaiData[SnssaiModelsToHex(*snssai)] = smPolicySnssaiData
	smPolicyDatBsonA := toBsonM(smPolicyData)
	smPolicyDatBsonA["ueId"] = "imsi-" + imsi
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, errPost := db.CommonDBClient.RestfulAPIPost(db.SmPolicyDataColl, filter, smPolicyDatBsonA)
	if errPost != nil {
		logger.DbLog.Warnln(errPost)
	}
}

func updateAmProvisionedData(snssai *openAPIModels.Snssai, qos *models.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, imsi string) {
	amData := openAPIModels.AccessAndMobilitySubscriptionData{
		Gpsis: []string{
			"msisdn-0900000000",
		},
		Nssai: &openAPIModels.Nssai{
			DefaultSingleNssais: []openAPIModels.Snssai{*snssai},
			SingleNssais:        []openAPIModels.Snssai{*snssai},
		},
		SubscribedUeAmbr: &openAPIModels.AmbrRm{
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

func updateSmProvisionedData(snssai *openAPIModels.Snssai, qos *models.DeviceGroupsIpDomainExpandedUeDnnQos, mcc, mnc, dnn, imsi string) {
	smData := openAPIModels.SessionManagementSubscriptionData{
		SingleNssai: snssai,
		DnnConfigurations: map[string]openAPIModels.DnnConfiguration{
			dnn: {
				PduSessionTypes: &openAPIModels.PduSessionTypes{
					DefaultSessionType:  openAPIModels.PduSessionType_IPV4,
					AllowedSessionTypes: []openAPIModels.PduSessionType{openAPIModels.PduSessionType_IPV4},
				},
				SscModes: &openAPIModels.SscModes{
					DefaultSscMode: openAPIModels.SscMode__1,
					AllowedSscModes: []openAPIModels.SscMode{
						"SSC_MODE_2",
						"SSC_MODE_3",
					},
				},
				SessionAmbr: &openAPIModels.Ambr{
					Downlink: convertToString(uint64(qos.DnnMbrDownlink)),
					Uplink:   convertToString(uint64(qos.DnnMbrUplink)),
				},
				Var5gQosProfile: &openAPIModels.SubscribedDefaultQos{
					Var5qi: 9,
					Arp: &openAPIModels.Arp{
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

func updateSmfSelectionProviosionedData(snssai *openAPIModels.Snssai, mcc, mnc, dnn, imsi string) {
	smfSelData := openAPIModels.SmfSelectionSubscriptionData{
		SubscribedSnssaiInfos: map[string]openAPIModels.SnssaiInfo{
			SnssaiModelsToHex(*snssai): {
				DnnInfos: []openAPIModels.DnnInfo{
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

func isDeviceGroupExistInSlice(deviceGroupName string) *models.Slice {
	for name, slice := range getSlices() {
		for _, dgName := range slice.SiteDeviceGroup {
			if dgName == deviceGroupName {
				logger.NMSLog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return slice
			}
		}
	}

	return nil
}

func getDeleteGroupsList(slice, prevSlice *models.Slice) (names []string) {
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

func updateDeviceGroupConfig(deviceGroupName string, deviceGroup *models.DeviceGroups, prevDeviceGroup *models.DeviceGroups) {
	slice := isDeviceGroupExistInSlice(deviceGroupName)
	if slice != nil {
		sVal, err := strconv.ParseUint(slice.SliceId.Sst, 10, 32)
		if err != nil {
			logger.DbLog.Errorf("Could not parse SST %v", slice.SliceId.Sst)
		}
		snssai := &openAPIModels.Snssai{
			Sd:  slice.SliceId.Sd,
			Sst: int32(sVal),
		}

		aimsis := getAddedImsisList(deviceGroup, prevDeviceGroup)
		for _, imsi := range aimsis {
			dnn := deviceGroup.IpDomainExpanded.Dnn
			updateAmPolicyData(imsi)
			updateSmPolicyData(snssai, dnn, imsi)
			updateAmProvisionedData(snssai, deviceGroup.IpDomainExpanded.UeDnnQos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, imsi)
			updateSmProvisionedData(snssai, deviceGroup.IpDomainExpanded.UeDnnQos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
			updateSmfSelectionProviosionedData(snssai, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
		}

		dimsis := getDeletedImsisList(deviceGroup, prevDeviceGroup)
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

func SnssaiModelsToHex(snssai openAPIModels.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}

func updateSMF() {
	networkSlices := make([]models.Slice, 0)
	networkSliceNames := ListNetworkSlices()
	for _, networkSliceName := range networkSliceNames {
		networkSlice := GetNetworkSliceByName2(networkSliceName)
		networkSlices = append(networkSlices, networkSlice)
	}
	deviceGroups := make([]models.DeviceGroups, 0)
	deviceGroupNames := ListDeviceGroups()
	for _, deviceGroupName := range deviceGroupNames {
		deviceGroup := GetDeviceGroupByName2(deviceGroupName)
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
