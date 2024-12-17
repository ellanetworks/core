package server

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	openAPIModels "github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/models"
	"github.com/yeastengine/ella/internal/util/httpwrapper"
)

var imsiData map[string]*openAPIModels.AuthenticationSubscription

func init() {
	imsiData = make(map[string]*openAPIModels.AuthenticationSubscription)
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

func GetDeviceGroups(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		deviceGroups, err := dbInstance.ListProfiles()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve device groups"})
			return
		}
		c.JSON(http.StatusOK, deviceGroups)
	}
}

func GetDeviceGroup(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("group-name")
		if !exists {
			logger.NmsLog.Errorf("group-name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing group-name parameter"})
			return
		}
		dbDeviceGroup, err := dbInstance.GetProfile(groupName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Unable to retrieve device group"})
			return
		}

		deviceGroup := models.DeviceGroups{
			DeviceGroupName: dbDeviceGroup.Name,
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
		imsis, err := dbDeviceGroup.GetImsis()
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to retrieve device group"})
			return
		}
		deviceGroup.Imsis = imsis
		c.JSON(http.StatusOK, deviceGroup)
	}
}

func PostDeviceGroup(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("group-name")
		if !exists {
			logger.NmsLog.Errorf("group-name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing group-name parameter"})
			return
		}
		_, err := dbInstance.GetProfile(groupName)
		if err == nil {
			logger.NmsLog.Warnf("Device Group [%v] already exists", groupName)
			c.JSON(http.StatusConflict, gin.H{"error": "Device Group already exists"})
			return
		}
		var request models.DeviceGroups
		s := strings.Split(c.GetHeader("Content-Type"), ";")
		switch s[0] {
		case "application/json":
			err = c.ShouldBindJSON(&request)
		}
		if err != nil {
			logger.NmsLog.Infof(" err %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
			return
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
		slice := isDeviceGroupExistInSlice(dbInstance, groupName)
		if slice != nil {
			sVal, err := strconv.ParseUint(slice.Sst, 10, 32)
			if err != nil {
				logger.NmsLog.Errorf("Could not parse SST %v", slice.Sst)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SST"})
				return
			}

			aimsis := getAddedImsisList(&procReq)
			for _, imsi := range aimsis {
				dnn := procReq.IpDomainExpanded.Dnn
				ueId := "imsi-" + imsi
				plmnId := slice.Mcc + slice.Mnc
				bitRateUplink := convertToString(uint64(procReq.IpDomainExpanded.UeDnnQos.DnnMbrUplink))
				bitRateDownlink := convertToString(uint64(procReq.IpDomainExpanded.UeDnnQos.DnnMbrDownlink))
				var5qi := 9
				priorityLevel := 8
				err = dbInstance.UpdateSubscriberProfile(ueId, dnn, slice.Sd, int32(sVal), plmnId, bitRateUplink, bitRateDownlink, var5qi, priorityLevel)
				if err != nil {
					logger.NmsLog.Warnf("Could not update subscriber %v", ueId)
					c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update subscriber"})
					return
				}
			}
		}
		dbDeviceGroup := &db.Profile{
			Name:           groupName,
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
		dbDeviceGroup.SetImsis(procReq.Imsis)
		err = dbInstance.CreateProfile(dbDeviceGroup)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create device group"})
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Created Device Group: %v", groupName)
		c.JSON(http.StatusCreated, gin.H{"message": "Device Group created successfully"})
	}
}

func DeleteDeviceGroup(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCorsHeader(c)
		groupName, exists := c.Params.Get("group-name")
		if !exists {
			logger.NmsLog.Errorf("group-name is missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing group-name parameter"})
			return
		}
		profile, err := dbInstance.GetProfile(groupName)
		if err != nil {
			logger.NmsLog.Warnf("Device Group [%v] not found", groupName)
			c.JSON(http.StatusNotFound, gin.H{"error": "Device Group not found"})
			return
		}
		deleteDeviceGroupConfig(dbInstance, profile)
		err = dbInstance.DeleteProfile(groupName)
		if err != nil {
			logger.NmsLog.Warnln(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete device group"})
			return
		}
		updateSMF(dbInstance)
		logger.NmsLog.Infof("Deleted Device Group: %v", groupName)
		c.JSON(http.StatusOK, gin.H{"message": "Device Group deleted successfully"})
	}
}

func getAddedImsisList(group *models.DeviceGroups) (aimsis []string) {
	for _, imsi := range group.Imsis {
		if imsiData[imsi] != nil {
			aimsis = append(aimsis, imsi)
		}
	}
	return
}

func deleteDeviceGroupConfig(dbInstance *db.Database, deviceGroup *db.Profile) {
	slice := isDeviceGroupExistInSlice(dbInstance, deviceGroup.Name)
	if slice != nil {
		dimsis, err := deviceGroup.GetImsis()
		if err != nil {
			logger.NmsLog.Warnln(err)
			return
		}
		for _, imsi := range dimsis {
			ueId := "imsi-" + imsi
			err = dbInstance.UpdateSubscriberProfile(ueId, "", "", 0, "", "", "", 0, 0)
			if err != nil {
				logger.NmsLog.Warnln(err)
			}
		}
	}
}

func isDeviceGroupExistInSlice(dbInstance *db.Database, deviceGroupName string) *db.NetworkSlice {
	dBSlices, err := dbInstance.ListNetworkSlices()
	if err != nil {
		logger.NmsLog.Warnln(err)
		return nil
	}
	for name, slice := range dBSlices {
		deviceGroups := slice.GetDeviceGroups()
		for _, dgName := range deviceGroups {
			if dgName == deviceGroupName {
				logger.NmsLog.Infof("Device Group [%v] is part of slice: %v", dgName, name)
				return &slice
			}
		}
	}

	return nil
}
