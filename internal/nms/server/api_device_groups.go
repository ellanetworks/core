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

func GetDeviceGroups(c *gin.Context) {
	setCorsHeader(c)
	deviceGroups, err := queries.ListProfiles()
	if err != nil {
		logger.NmsLog.Warnln(err)
		c.JSON(http.StatusInternalServerError, nil)
		return
	}
	c.JSON(http.StatusOK, deviceGroups)
}

func GetDeviceGroupByName(c *gin.Context) {
	setCorsHeader(c)
	dbDeviceGroup := queries.GetProfile(c.Param("group-name"))
	if dbDeviceGroup.Name == "" {
		c.JSON(http.StatusNotFound, nil)
		return
	}
	deviceGroup := models.DeviceGroups{
		DeviceGroupName: dbDeviceGroup.Name,
		Imsis:           dbDeviceGroup.Imsis,
		IpDomainExpanded: models.DeviceGroupsIpDomainExpanded{
			UeIpPool:     dbDeviceGroup.UeIpPool,
			DnsPrimary:   dbDeviceGroup.DnsPrimary,
			DnsSecondary: dbDeviceGroup.DnsSecondary,
			UeDnnQos: &models.DeviceGroupsIpDomainExpandedUeDnnQos{
				DnnMbrDownlink: dbDeviceGroup.DnnMbrDownlink,
				DnnMbrUplink:   dbDeviceGroup.DnnMbrUplink,
				BitrateUnit:    dbDeviceGroup.BitrateUnit,
				TrafficClass: &models.TrafficClassInfo{
					Name: dbDeviceGroup.Name,
					Qci:  dbDeviceGroup.Qci,
					Arp:  dbDeviceGroup.Arp,
					Pdb:  dbDeviceGroup.Pdb,
					Pelr: dbDeviceGroup.Pelr,
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
	dbDeviceGroup := queries.GetProfile(groupName)
	deviceGroup := &models.DeviceGroups{
		DeviceGroupName: dbDeviceGroup.Name,
		Imsis:           dbDeviceGroup.Imsis,
		IpDomainExpanded: models.DeviceGroupsIpDomainExpanded{
			UeIpPool:     dbDeviceGroup.UeIpPool,
			DnsPrimary:   dbDeviceGroup.DnsPrimary,
			DnsSecondary: dbDeviceGroup.DnsSecondary,
			Mtu:          dbDeviceGroup.Mtu,
			UeDnnQos: &models.DeviceGroupsIpDomainExpandedUeDnnQos{
				DnnMbrDownlink: dbDeviceGroup.DnnMbrDownlink,
				DnnMbrUplink:   dbDeviceGroup.DnnMbrUplink,
				BitrateUnit:    dbDeviceGroup.BitrateUnit,
				TrafficClass: &models.TrafficClassInfo{
					Name: dbDeviceGroup.Name,
					Qci:  dbDeviceGroup.Qci,
					Arp:  dbDeviceGroup.Arp,
					Pdb:  dbDeviceGroup.Pdb,
					Pelr: dbDeviceGroup.Pelr,
				},
			},
		},
	}
	err := queries.DeleteProfile(groupName)
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
		sVal, err := strconv.ParseUint(slice.Sst, 10, 32)
		if err != nil {
			logger.NmsLog.Errorf("Could not parse SST %v", slice.Sst)
		}

		aimsis := getAddedImsisList(&procReq)
		for _, imsi := range aimsis {
			dnn := procReq.IpDomainExpanded.Dnn
			ueId := "imsi-" + imsi
			subscriber, err := queries.GetSubscriber(ueId)
			if err != nil {
				logger.NmsLog.Warnf("Could not get subscriber %v", ueId)
				continue
			}
			subscriber.Dnn = dnn
			subscriber.Sd = slice.Sd
			subscriber.Sst = int32(sVal)
			subscriber.PlmnID = slice.Mcc + slice.Mnc
			subscriber.BitRateUplink = convertToString(uint64(procReq.IpDomainExpanded.UeDnnQos.DnnMbrUplink))
			subscriber.BitRateDownlink = convertToString(uint64(procReq.IpDomainExpanded.UeDnnQos.DnnMbrDownlink))
			subscriber.Var5qi = 9
			subscriber.PriorityLevel = 8
			err = queries.CreateSubscriber(subscriber)
			if err != nil {
				logger.NmsLog.Warnf("Could not create subscriber %v", ueId)
				continue
			}
		}
	}
	dbDeviceGroup := &dbModels.Profile{
		Name:           groupName,
		Imsis:          procReq.Imsis,
		UeIpPool:       procReq.IpDomainExpanded.UeIpPool,
		DnsPrimary:     procReq.IpDomainExpanded.DnsPrimary,
		DnsSecondary:   procReq.IpDomainExpanded.DnsSecondary,
		Mtu:            procReq.IpDomainExpanded.Mtu,
		DnnMbrDownlink: procReq.IpDomainExpanded.UeDnnQos.DnnMbrDownlink,
		DnnMbrUplink:   procReq.IpDomainExpanded.UeDnnQos.DnnMbrUplink,
		BitrateUnit:    procReq.IpDomainExpanded.UeDnnQos.BitrateUnit,
		Qci:            procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Qci,
		Arp:            procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Arp,
		Pdb:            procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pdb,
		Pelr:           procReq.IpDomainExpanded.UeDnnQos.TrafficClass.Pelr,
	}
	err = queries.CreateProfile(dbDeviceGroup)
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
			ueId := "imsi-" + imsi
			subscriber, err := queries.GetSubscriber(ueId)
			if err != nil {
				logger.NmsLog.Warnln(err)
				continue
			}
			subscriber.BitRateUplink = ""
			subscriber.BitRateDownlink = ""
			subscriber.Var5qi = 0
			subscriber.PriorityLevel = 0
			err = queries.CreateSubscriber(subscriber)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
		}
	}
}

func isDeviceGroupExistInSlice(deviceGroupName string) *dbModels.NetworkSlice {
	dBSlices := queries.ListNetworkSlices()
	for name, slice := range dBSlices {
		for _, dgName := range slice.DeviceGroups {
			if dgName == deviceGroupName {
				logger.NmsLog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return slice
			}
		}
	}

	return nil
}
