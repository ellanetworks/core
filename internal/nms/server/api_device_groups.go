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
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nms/logger"
	"github.com/yeastengine/ella/internal/nms/models"
)

func GetDeviceGroups(c *gin.Context) {
	setCorsHeader(c)
	deviceGroups, err := db.ListDeviceGroupNames()
	if err != nil {
		logger.DbLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	c.JSON(http.StatusOK, deviceGroups)
}

func GetDeviceGroupByName(c *gin.Context) {
	setCorsHeader(c)
	dbDeviceGroup := db.GetDeviceGroupByName(c.Param("group-name"))
	if dbDeviceGroup.DeviceGroupName == "" {
		c.JSON(http.StatusNotFound, nil)
		return
	}
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

	c.JSON(http.StatusOK, deviceGroup)
}

func DeviceGroupGroupNameDelete(c *gin.Context) {
	if ret := DeviceGroupDeleteHandler(c); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func DeviceGroupGroupNamePut(c *gin.Context) {
	if ret := DeviceGroupPostHandler(c, models.Put_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func DeviceGroupGroupNamePatch(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func DeviceGroupGroupNamePost(c *gin.Context) {
	if ret := DeviceGroupPostHandler(c, models.Post_op); ret {
		c.JSON(http.StatusOK, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{})
	}
}

func DeviceGroupDeleteHandler(c *gin.Context) bool {
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		logger.ConfigLog.Errorf("group-name is missing")
		return false
	}
	dbDeviceGroup := db.GetDeviceGroupByName(groupName)
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
			Mtu:          dbDeviceGroup.IpDomainExpanded.Mtu,
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
	success := db.DeleteDeviceGroup(groupName)
	if !success {
		logger.NMSLog.Warnf("Device Group [%v] not found", groupName)
		return false
	}
	deleteDeviceGroupConfig(deviceGroup)
	updateSMF()
	logger.ConfigLog.Infof("Deleted Device Group: %v", groupName)
	return true
}

func DeviceGroupPostHandler(c *gin.Context, msgOp int) bool {
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		logger.ConfigLog.Errorf("group-name is missing")
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
		logger.ConfigLog.Infof(" err %v", err)
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
	dbDeviceGroup := &db.DeviceGroup{
		DeviceGroupName: groupName,
		Imsis:           procReq.Imsis,
		SiteInfo:        procReq.SiteInfo,
		IpDomainName:    procReq.IpDomainName,
		IpDomainExpanded: db.DeviceGroupsIpDomainExpanded{
			Dnn:          procReq.IpDomainExpanded.Dnn,
			UeIpPool:     procReq.IpDomainExpanded.UeIpPool,
			DnsPrimary:   procReq.IpDomainExpanded.DnsPrimary,
			DnsSecondary: procReq.IpDomainExpanded.DnsSecondary,
			Mtu:          procReq.IpDomainExpanded.Mtu,
			UeDnnQos: &db.DeviceGroupsIpDomainExpandedUeDnnQos{
				DnnMbrDownlink: procReq.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
				DnnMbrUplink:   procReq.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
				BitrateUnit:    procReq.IpDomainExpanded.UeDnnQos.BitrateUnit,
				TrafficClass: &db.TrafficClassInfo{
					Name: procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Name,
					Qci:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
					Arp:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
					Pdb:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
					Pelr: procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
				},
			},
		},
	}
	err = db.CreateDeviceGroup(dbDeviceGroup)
	if err != nil {
		logger.DbLog.Warnln(err)
		return false
	}
	updateSMF()
	logger.ConfigLog.Infof("Created Device Group: %v", groupName)
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
			err := db.DeleteAmPolicy(imsi)
			if err != nil {
				logger.DbLog.Warnln(err)
			}
			err = db.DeleteSmPolicy(imsi)
			if err != nil {
				logger.DbLog.Warnln(err)
			}
			err = db.DeleteAmData(imsi, mcc, mnc)
			if err != nil {
				logger.DbLog.Warnln(err)
			}
			err = db.DeleteSmData(imsi, mcc, mnc)
			if err != nil {
				logger.DbLog.Warnln(err)
			}
			err = db.DeleteSmfSelf(imsi, mcc, mnc)
			if err != nil {
				logger.DbLog.Warnln(err)
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
