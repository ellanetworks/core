package server

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	dbModels "github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/db/queries"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
)

func GetDeviceGroups(c *gin.Context) {
	setCorsHeader(c)
	deviceGroups, err := queries.ListDeviceGroupNames()
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	c.JSON(http.StatusOK, deviceGroups)
}

func GetDeviceGroupByName(c *gin.Context) {
	setCorsHeader(c)
	dbDeviceGroup := queries.GetDeviceGroupByName(c.Param("group-name"))
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
		logger.NmsLog.Errorf("group-name is missing")
		return false
	}
	dbDeviceGroup := queries.GetDeviceGroupByName(groupName)
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
	err := queries.DeleteDeviceGroup(groupName)
	if err != nil {
		logger.NmsLog.Warnf("Device Group [%v] not found", groupName)
		return false
	}
	deleteDeviceGroupConfig(deviceGroup)
	updateSMF()
	logger.NmsLog.Infof("Deleted Device Group: %v", groupName)
	return true
}

func DeviceGroupPostHandler(c *gin.Context, msgOp int) bool {
	groupName, exists := c.Params.Get("group-name")
	if !exists {
		logger.NmsLog.Errorf("group-name is missing")
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
		logger.NmsLog.Infof(" err %v", err)
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
			logger.NmsLog.Errorf("Could not parse SST %v", slice.SliceId.Sst)
		}
		snssai := &dbModels.Snssai{
			Sd:  slice.SliceId.Sd,
			Sst: int32(sVal),
		}

		aimsis := getAddedImsisList(&procReq)
		for _, imsi := range aimsis {
			dnn := procReq.IpDomainExpanded.Dnn
			err := queries.CreateAmPolicyData(imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
				return false
			}
			err = queries.CreateSmPolicyData(snssai, dnn, imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
				return false
			}
			qos := &dbModels.DeviceGroupsIpDomainExpandedUeDnnQos{
				DnnMbrDownlink: procReq.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
				DnnMbrUplink:   procReq.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
				BitrateUnit:    procReq.IpDomainExpanded.UeDnnQos.BitrateUnit,
				TrafficClass: &dbModels.TrafficClassInfo{
					Name: procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Name,
					Qci:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
					Arp:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
					Pdb:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
					Pelr: procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
				},
			}
			err = queries.CreateAmProvisionedData(snssai, qos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
				return false
			}
			err = queries.CreateSmProvisionedData(snssai, qos, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
				return false
			}
			err = queries.CreateSmfSelectionProviosionedData(snssai, slice.SiteInfo.Plmn.Mcc, slice.SiteInfo.Plmn.Mnc, dnn, imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
				return false
			}
		}
	}
	dbDeviceGroup := &dbModels.DeviceGroup{
		DeviceGroupName: groupName,
		Imsis:           procReq.Imsis,
		SiteInfo:        procReq.SiteInfo,
		IpDomainName:    procReq.IpDomainName,
		IpDomainExpanded: dbModels.DeviceGroupsIpDomainExpanded{
			Dnn:          procReq.IpDomainExpanded.Dnn,
			UeIpPool:     procReq.IpDomainExpanded.UeIpPool,
			DnsPrimary:   procReq.IpDomainExpanded.DnsPrimary,
			DnsSecondary: procReq.IpDomainExpanded.DnsSecondary,
			Mtu:          procReq.IpDomainExpanded.Mtu,
			UeDnnQos: &dbModels.DeviceGroupsIpDomainExpandedUeDnnQos{
				DnnMbrDownlink: procReq.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
				DnnMbrUplink:   procReq.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
				BitrateUnit:    procReq.IpDomainExpanded.UeDnnQos.BitrateUnit,
				TrafficClass: &dbModels.TrafficClassInfo{
					Name: procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Name,
					Qci:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
					Arp:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
					Pdb:  procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
					Pelr: procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
				},
			},
		},
	}
	err = queries.CreateDeviceGroup(dbDeviceGroup)
	if err != nil {
		logger.NmsLog.Warnln(err)
		return false
	}
	updateSMF()
	logger.NmsLog.Infof("Created Device Group: %v", groupName)
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
			err := queries.DeleteAmPolicy(imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
			err = queries.DeleteSmPolicy(imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
			err = queries.DeleteAmData(imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
			err = queries.DeleteSmData(imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
			err = queries.DeleteSmfSelection(imsi)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
		}
	}
}

func isDeviceGroupExistInSlice(deviceGroupName string) *dbModels.Slice {
	dBSlices := queries.ListNetworkSlices()
	for name, slice := range dBSlices {
		for _, dgName := range slice.SiteDeviceGroup {
			if dgName == deviceGroupName {
				logger.NmsLog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return slice
			}
		}
	}

	return nil
}
