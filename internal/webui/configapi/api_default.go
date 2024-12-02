package configapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/webui/backend/logger"
	"github.com/yeastengine/ella/internal/webui/configmodels"
	"github.com/yeastengine/ella/internal/webui/dbadapter"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	devGroupDataColl = "webconsoleData.snapshots.devGroupData"
	sliceDataColl    = "webconsoleData.snapshots.sliceData"
)

func ListDeviceGroups() []string {
	var deviceGroups []string = make([]string, 0)
	rawDeviceGroups, errGetMany := dbadapter.CommonDBClient.RestfulAPIGetMany(devGroupDataColl, bson.M{})
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
	logger.WebUILog.Infoln("Get all Device Groups")
	deviceGroups := ListDeviceGroups()
	c.JSON(http.StatusOK, deviceGroups)
}

func GetDeviceGroupByName2(groupName string) configmodels.DeviceGroups {
	var deviceGroup configmodels.DeviceGroups
	filter := bson.M{"group-name": groupName}
	rawDeviceGroup, err := dbadapter.CommonDBClient.RestfulAPIGetOne(devGroupDataColl, filter)
	if err != nil {
		logger.DbLog.Warnln(err)
	}
	json.Unmarshal(mapToByte(rawDeviceGroup), &deviceGroup)
	return deviceGroup
}

func GetDeviceGroupByName(c *gin.Context) {
	setCorsHeader(c)
	logger.WebUILog.Infoln("Get Device Group by name")
	deviceGroup := GetDeviceGroupByName2(c.Param("group-name"))
	if deviceGroup.DeviceGroupName == "" {
		c.JSON(http.StatusNotFound, nil)
	} else {
		c.JSON(http.StatusOK, deviceGroup)
	}
}

func DeviceGroupGroupNameDelete(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNameDelete")
	if ret := DeviceGroupDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func DeviceGroupGroupNamePut(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNamePut")
	if ret := DeviceGroupPostHandler(c, configmodels.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func DeviceGroupGroupNamePatch(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNamePatch")
	c.JSON(http.StatusOK, gin.H{})
}

func DeviceGroupGroupNamePost(c *gin.Context) {
	logger.ConfigLog.Debugf("DeviceGroupGroupNamePost")
	if ret := DeviceGroupPostHandler(c, configmodels.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func ListNetworkSlices() []string {
	var networkSlices []string = make([]string, 0)
	rawNetworkSlices, errGetMany := dbadapter.CommonDBClient.RestfulAPIGetMany(sliceDataColl, bson.M{})
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
	logger.WebUILog.Infoln("List Network Slices")
	networkSlices := ListNetworkSlices()
	c.JSON(http.StatusOK, networkSlices)
}

func GetNetworkSliceByName2(sliceName string) configmodels.Slice {
	var networkSlice configmodels.Slice
	filter := bson.M{"slice-name": sliceName}
	rawNetworkSlice, err := dbadapter.CommonDBClient.RestfulAPIGetOne(sliceDataColl, filter)
	if err != nil {
		logger.DbLog.Warnln(err)
	}
	json.Unmarshal(mapToByte(rawNetworkSlice), &networkSlice)
	return networkSlice
}

func GetNetworkSliceByName(c *gin.Context) {
	setCorsHeader(c)
	logger.WebUILog.Infoln("Get Network Slice by name")
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

func NetworkSliceSliceNamePost(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, configmodels.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func NetworkSliceSliceNamePut(c *gin.Context) {
	if ret := NetworkSlicePostHandler(c, configmodels.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}
