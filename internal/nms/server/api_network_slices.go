package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	openAPIModels "github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/db"
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

var imsiData map[string]*openAPIModels.AuthenticationSubscription

func init() {
	imsiData = make(map[string]*openAPIModels.AuthenticationSubscription)
}

func ListNetworkSlices() []string {
	var networkSlices []string = make([]string, 0)
	rawNetworkSlices, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.SliceDataColl, bson.M{})
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	for _, rawNetworkSlice := range rawNetworkSlices {
		if rawNetworkSlice["slice-name"] == nil {
			logger.ConfigLog.Errorf("slice-name is nil")
			continue
		}
		networkSlices = append(networkSlices, rawNetworkSlice["slice-name"].(string))
	}
	return networkSlices
}

func GetNetworkSlices(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("List Network Slices")
	networkSlices := ListNetworkSlices()
	c.JSON(http.StatusOK, networkSlices)
}

func GetNetworkSliceByName2(sliceName string) models.Slice {
	var networkSlice models.Slice
	filter := bson.M{"slice-name": sliceName}
	rawNetworkSlice, err := db.CommonDBClient.RestfulAPIGetOne(db.SliceDataColl, filter)
	if err != nil {
		logger.DbLog.Warnln(err)
	}
	json.Unmarshal(mapToByte(rawNetworkSlice), &networkSlice)
	return networkSlice
}

func GetNetworkSliceByName(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Get Network Slice by name")
	networkSlice := GetNetworkSliceByName2(c.Param("slice-name"))
	if networkSlice.SliceName == "" {
		c.JSON(http.StatusNotFound, nil)
	} else {
		c.JSON(http.StatusOK, networkSlice)
	}
}

// NetworkSliceSliceNameDelete -
func NetworkSliceSliceNameDelete(c *gin.Context) {
	if ret := NetworkSliceDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

// NetworkSliceSliceNamePost -
func NetworkSliceSliceNamePost(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, models.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

// NetworkSliceSliceNamePut -
func NetworkSliceSliceNamePut(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, models.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
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

func NetworkSliceDeleteHandler(c *gin.Context) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		logger.ConfigLog.Errorf("slice-name is missing")
		return false
	}
	prevSlice := getSliceByName(sliceName)
	filter := bson.M{"slice-name": sliceName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.SliceDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	dgnames := getDeleteGroupsList(nil, prevSlice)
	for _, dgname := range dgnames {
		devGroupConfig := db.GetDeviceGroupByName(dgname)
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
	logger.ConfigLog.Infof("Deleted Network Slice: %v", sliceName)
	return true
}

func NetworkSlicePostHandler(c *gin.Context, msgOp int) bool {
	sliceName, exists := c.Params.Get("slice-name")
	if !exists {
		logger.ConfigLog.Errorf("slice-name is missing")
		return false
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
		dbDeviceGroup := db.GetDeviceGroupByName(dgName)
		deviceGroup := &models.DeviceGroups{
			DeviceGroupName: dbDeviceGroup.DeviceGroupName,
			Imsis:           dbDeviceGroup.Imsis,
			SiteInfo:        dbDeviceGroup.SiteInfo,
			IpDomainName:    dbDeviceGroup.IpDomainName,
			IpDomainExpanded: models.DeviceGroupsIpDomainExpanded{
				Dnn:          dbDeviceGroup.IpDomainExpanded.Dnn,
				UeIpPool:     dbDeviceGroup.IpDomainExpanded.UeIpPool,
				DnsPrimary:   dbDeviceGroup.IpDomainExpanded.DnsPrimary,
				DnsSecondary: dbDeviceGroup.IpDomainExpanded.DnsSecondary,
				UeDnnQos: &models.DeviceGroupsIpDomainExpandedUeDnnQos{
					DnnMbrDownlink: dbDeviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
					DnnMbrUplink:   dbDeviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
					BitrateUnit:    dbDeviceGroup.IpDomainExpanded.UeDnnQos.BitrateUnit,
					TrafficClass: &models.TrafficClassInfo{
						Name: dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Name,
						Qci:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
						Arp:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
						Pdb:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
						Pelr: dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
					},
				},
			},
		}
		if deviceGroup != nil {
			for _, imsi := range deviceGroup.Imsis {
				dnn := deviceGroup.IpDomainExpanded.Dnn
				mcc := procReq.SiteInfo.Plmn.Mcc
				mnc := procReq.SiteInfo.Plmn.Mnc
				updateAmPolicyData(imsi)
				updateSmPolicyData(snssai, dnn, imsi)
				updateAmProvisionedData(snssai, deviceGroup.IpDomainExpanded.UeDnnQos, mcc, mnc, imsi)
				updateSmProvisionedData(snssai, deviceGroup.IpDomainExpanded.UeDnnQos, mcc, mnc, dnn, imsi)
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
	logger.ConfigLog.Infof("Created Network Slice: %v", sliceName)
	return true
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
	deviceGroupNames := db.ListDeviceGroupNames()
	for _, deviceGroupName := range deviceGroupNames {
		dbDeviceGroup := db.GetDeviceGroupByName(deviceGroupName)
		deviceGroup := models.DeviceGroups{
			DeviceGroupName: dbDeviceGroup.DeviceGroupName,
			Imsis:           dbDeviceGroup.Imsis,
			SiteInfo:        dbDeviceGroup.SiteInfo,
			IpDomainName:    dbDeviceGroup.IpDomainName,
			IpDomainExpanded: models.DeviceGroupsIpDomainExpanded{
				Dnn:          dbDeviceGroup.IpDomainExpanded.Dnn,
				UeIpPool:     dbDeviceGroup.IpDomainExpanded.UeIpPool,
				DnsPrimary:   dbDeviceGroup.IpDomainExpanded.DnsPrimary,
				DnsSecondary: dbDeviceGroup.IpDomainExpanded.DnsSecondary,
				UeDnnQos: &models.DeviceGroupsIpDomainExpandedUeDnnQos{
					DnnMbrDownlink: dbDeviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
					DnnMbrUplink:   dbDeviceGroup.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
					BitrateUnit:    dbDeviceGroup.IpDomainExpanded.UeDnnQos.BitrateUnit,
					TrafficClass: &models.TrafficClassInfo{
						Name: dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Name,
						Qci:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
						Arp:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
						Pdb:  dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
						Pelr: dbDeviceGroup.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
					},
				},
			},
		}
		deviceGroups = append(deviceGroups, deviceGroup)
	}
	context.UpdateSMFContext(networkSlices, deviceGroups)
}
