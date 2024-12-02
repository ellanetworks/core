package server

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
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
	logger.ConfigLog.Debugf("Received NetworkSliceSliceNameDelete ")
	if ret := NetworkSliceDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

// NetworkSliceSliceNamePost -
func NetworkSliceSliceNamePost(c *gin.Context) {
	logger.ConfigLog.Debugf("Received NetworkSliceSliceNamePost ")
	if ret := NetworkSlicePostHandler(c, models.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

// NetworkSliceSliceNamePut -
func NetworkSliceSliceNamePut(c *gin.Context) {
	logger.ConfigLog.Debugf("Received NetworkSliceSliceNamePut ")
	if ret := NetworkSlicePostHandler(c, models.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}
