package server

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	openAPIModels "github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/nms/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"go.mongodb.org/mongo-driver/bson"
)

func ListDeviceGroups() []string {
	var deviceGroups []string = make([]string, 0)
	rawDeviceGroups, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.DevGroupDataColl, bson.M{})
	if errGetMany != nil {
		logger.DbLog.Warnln(errGetMany)
	}
	for _, rawDeviceGroup := range rawDeviceGroups {
		deviceGroups = append(deviceGroups, rawDeviceGroup["group-name"].(string))
	}
	return deviceGroups
}

func GetDeviceGroups(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Get all Device Groups")
	deviceGroups := ListDeviceGroups()
	c.JSON(http.StatusOK, deviceGroups)
}

func GetDeviceGroupByName2(groupName string) models.DeviceGroups {
	var deviceGroup models.DeviceGroups
	filter := bson.M{"group-name": groupName}
	rawDeviceGroup, err := db.CommonDBClient.RestfulAPIGetOne(db.DevGroupDataColl, filter)
	if err != nil {
		logger.DbLog.Warnln(err)
	}
	json.Unmarshal(mapToByte(rawDeviceGroup), &deviceGroup)
	return deviceGroup
}

func GetDeviceGroupByName(c *gin.Context) {
	setCorsHeader(c)
	logger.NMSLog.Infoln("Get Device Group by name")
	deviceGroup := GetDeviceGroupByName2(c.Param("group-name"))
	if deviceGroup.DeviceGroupName == "" {
		c.JSON(http.StatusNotFound, nil)
	} else {
		c.JSON(http.StatusOK, deviceGroup)
	}
}

// DeviceGroupGroupNameDelete -
func DeviceGroupGroupNameDelete(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNameDelete")
	if ret := DeviceGroupDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

// DeviceGroupGroupNamePut -
func DeviceGroupGroupNamePut(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNamePut")
	if ret := DeviceGroupPostHandler(c, models.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

// DeviceGroupGroupNamePatch -
func DeviceGroupGroupNamePatch(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNamePatch")
	c.JSON(http.StatusOK, gin.H{})
}

// DeviceGroupGroupNamePost -
func DeviceGroupGroupNamePost(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNamePost")
	if ret := DeviceGroupPostHandler(c, models.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func DeviceGroupDeleteHandler(c *gin.Context) bool {
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		configLog.Errorf("group-name is missing")
		return false
	}
	deviceGroup := getDeviceGroupByName(groupName)
	filter := bson.M{"group-name": groupName}
	errDelOne := db.CommonDBClient.RestfulAPIDeleteOne(db.DevGroupDataColl, filter)
	if errDelOne != nil {
		logger.DbLog.Warnln(errDelOne)
	}
	deleteDeviceGroupConfig(deviceGroup)
	updateSMF()
	configLog.Infof("Deleted Device Group: %v", groupName)
	return true
}

func DeviceGroupPostHandler(c *gin.Context, msgOp int) bool {
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		configLog.Errorf("group-name is missing")
		return false
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
	slice := isDeviceGroupExistInSlice(groupName)
	if slice != nil {
		sVal, err := strconv.ParseUint(slice.SliceId.Sst, 10, 32)
		if err != nil {
			logger.DbLog.Errorf("Could not parse SST %v", slice.SliceId.Sst)
		}
		snssai := &openAPIModels.Snssai{
			Sd:  slice.SliceId.Sd,
			Sst: int32(sVal),
		}

		aimsis := getAddedImsisList(&procReq)
		for _, imsi := range aimsis {
			dnn := procReq.IpDomainExpanded.Dnn
			updateAmPolicyData(imsi)
			updateSmPolicyData(snssai, dnn, imsi)
			updateAmProvisionedData(snssai, procReq.IpDomainExpanded.UeDnnQos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, imsi)
			updateSmProvisionedData(snssai, procReq.IpDomainExpanded.UeDnnQos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
			updateSmfSelectionProviosionedData(snssai, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
		}
	}
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

func getAddedImsisList(group *models.DeviceGroups) (aimsis []string) {
	for _, imsi := range group.Imsis {
		if imsiData[imsi] != nil {
			aimsis = append(aimsis, imsi)
		}
	}

	return
}

func deleteDeviceGroupConfig(deviceGroup *models.DeviceGroups) {
	slice := isDeviceGroupExistInSlice(deviceGroup.DeviceGroupName)
	if slice != nil {
		dimsis := deviceGroup.Imsis
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
